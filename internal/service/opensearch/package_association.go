// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package opensearch

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	tfslices "github.com/hashicorp/terraform-provider-aws/internal/slices"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @SDKResource("aws_opensearch_package_association")
func ResourcePackageAssociation() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourcePackageAssociationCreate,
		ReadWithoutTimeout:   resourcePackageAssociationRead,
		DeleteWithoutTimeout: resourcePackageAssociationDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			names.AttrDomainName: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"package_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"reference_path": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourcePackageAssociationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).OpenSearchConn(ctx)

	domainName := d.Get(names.AttrDomainName).(string)
	packageID := d.Get("package_id").(string)
	id := fmt.Sprintf("%s-%s", domainName, packageID)
	input := &opensearchservice.AssociatePackageInput{
		DomainName: aws.String(domainName),
		PackageID:  aws.String(packageID),
	}
	_, err := tfresource.RetryWhenAWSErrMessageContains(ctx, propagationTimeout,
		func() (interface{}, error) {
			return conn.AssociatePackageWithContext(ctx, input)
		},
		opensearchservice.ErrCodeValidationException,
		"A change/update is in progress",
	)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating OpenSearch Package Association (%s): %s", id, err)
	}

	d.SetId(id)

	return append(diags, resourcePackageAssociationRead(ctx, d, meta)...)
}

func resourcePackageAssociationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).OpenSearchConn(ctx)

	domainName := d.Get(names.AttrDomainName).(string)
	packageID := d.Get("package_id").(string)
	pkgAssociation, err := FindPackageAssociationByTwoPartKey(ctx, conn, domainName, packageID)

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] OpenSearch Package Association (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OpenSearch Package Association (%s): %s", d.Id(), err)
	}

	d.Set(names.AttrDomainName, pkgAssociation.DomainName)
	d.Set("package_id", pkgAssociation.PackageID)
	d.Set("reference_path", pkgAssociation.ReferencePath)

	return diags
}

func resourcePackageAssociationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).OpenSearchConn(ctx)

	log.Printf("[DEBUG] Deleting OpenSearch Package Association: %s", d.Id())
	domainName := d.Get(names.AttrDomainName).(string)
	packageID := d.Get("package_id").(string)

	_, err := tfresource.RetryWhenAWSErrMessageContains(ctx, propagationTimeout,
		func() (interface{}, error) {
			return conn.DissociatePackageWithContext(ctx, &opensearchservice.DissociatePackageInput{
				DomainName: aws.String(domainName),
				PackageID:  aws.String(packageID),
			})
		},
		opensearchservice.ErrCodeValidationException,
		"A change/update is in progress",
	)

	if tfawserr.ErrCodeEquals(err, opensearchservice.ErrCodeResourceNotFoundException) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting OpenSearch Package Association (%s): %s", d.Id(), err)
	}

	return diags
}

func FindPackageAssociationByTwoPartKey(ctx context.Context, conn *opensearchservice.OpenSearchService, domainName, packageID string) (*opensearchservice.DomainPackageDetails, error) {
	input := &opensearchservice.ListPackagesForDomainInput{
		DomainName: aws.String(domainName),
	}
	filter := func(v *opensearchservice.DomainPackageDetails) bool {
		return aws.StringValue(v.PackageID) == packageID
	}

	return findPackageAssociation(ctx, conn, input, filter)
}

func findPackageAssociation(ctx context.Context, conn *opensearchservice.OpenSearchService, input *opensearchservice.ListPackagesForDomainInput, filter tfslices.Predicate[*opensearchservice.DomainPackageDetails]) (*opensearchservice.DomainPackageDetails, error) {
	output, err := findPackageAssociations(ctx, conn, input, filter)

	if err != nil {
		return nil, err
	}

	return tfresource.AssertSinglePtrResult(output)
}

func findPackageAssociations(ctx context.Context, conn *opensearchservice.OpenSearchService, input *opensearchservice.ListPackagesForDomainInput, filter tfslices.Predicate[*opensearchservice.DomainPackageDetails]) ([]*opensearchservice.DomainPackageDetails, error) {
	var output []*opensearchservice.DomainPackageDetails

	err := conn.ListPackagesForDomainPagesWithContext(ctx, input, func(page *opensearchservice.ListPackagesForDomainOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, v := range page.DomainPackageDetailsList {
			if v != nil && filter(v) {
				output = append(output, v)
			}
		}

		return !lastPage
	})

	if tfawserr.ErrCodeEquals(err, opensearchservice.ErrCodeResourceNotFoundException) {
		return nil, &retry.NotFoundError{
			LastError:   err,
			LastRequest: input,
		}
	}

	if err != nil {
		return nil, err
	}

	return output, nil
}

func statusPackageAssociation(ctx context.Context, conn *opensearchservice.OpenSearchService, domainName, packageID string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		output, err := FindPackageAssociationByTwoPartKey(ctx, conn, domainName, packageID)

		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return output, aws.StringValue(output.DomainPackageStatus), nil
	}
}
