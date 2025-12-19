// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"fmt"

	"storj.io/storj/private/api"
	"storj.io/storj/satellite/payments"
)

// ProductInfo contains the price model of a product.
type ProductInfo struct {
	ProductID           int32  `json:"productID"`
	ProductName         string `json:"productName"`
	StorageMBMonthCents string `json:"storageMBMonthCents"`
	EgressMBCents       string `json:"egressMBCents"`
	SegmentMonthCents   string `json:"segmentMonthCents"`
	EgressDiscountRatio string `json:"egressDiscountRatio"`
}

// MiniInfo returns a subset of product pricing information.
func (p ProductInfo) MiniInfo() MiniProductInfo {
	return MiniProductInfo{
		ProductName:         fmt.Sprintf("(%d) - %s", p.ProductID, p.ProductName),
		StorageMBMonthCents: p.StorageMBMonthCents,
		EgressMBCents:       p.EgressMBCents,
		SegmentMonthCents:   p.SegmentMonthCents,
		EgressDiscountRatio: p.EgressDiscountRatio,
	}
}

// MiniProductInfo contains a subset of product pricing information.
type MiniProductInfo struct {
	ProductName         string `json:"productName"`
	StorageMBMonthCents string `json:"storageMBMonthCents"`
	EgressMBCents       string `json:"egressMBCents"`
	SegmentMonthCents   string `json:"segmentMonthCents"`
	EgressDiscountRatio string `json:"egressDiscountRatio"`
}

// GetProducts returns information about available products.
func (s *Service) GetProducts(ctx context.Context) ([]ProductInfo, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	infos := make([]ProductInfo, 0, len(s.placement))
	for _, product := range s.products {
		infos = append(infos, getProductInfo(product))
	}

	return infos, api.HTTPError{}
}

func (s *Service) getProductByID(productID int32) (ProductInfo, error) {
	product, exists := s.products[productID]
	if !exists {
		return ProductInfo{}, fmt.Errorf("product with ID %d not found", productID)
	}

	return getProductInfo(product), nil
}

func getProductInfo(product payments.ProductUsagePriceModel) ProductInfo {
	return ProductInfo{
		ProductID:           product.ProductID,
		ProductName:         product.ProductName,
		StorageMBMonthCents: product.StorageMBMonthCents.String(),
		EgressMBCents:       product.EgressMBCents.String(),
		SegmentMonthCents:   product.SegmentMonthCents.String(),
		EgressDiscountRatio: fmt.Sprintf("%.2f", product.EgressDiscountRatio),
	}
}
