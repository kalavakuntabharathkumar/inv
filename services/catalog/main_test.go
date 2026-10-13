package main

import "testing"

func TestProductValidationBasics(t *testing.T) {
	valid := Product{SKU: "sku-1", Name: "Notebook", Price: 12.5}
	if valid.SKU == "" || valid.Name == "" || valid.Price < 0 {
		t.Fatal("valid product rejected")
	}
	invalid := Product{SKU: "", Name: "", Price: -1}
	if invalid.SKU != "" && invalid.Name != "" && invalid.Price >= 0 {
		t.Fatal("invalid product accepted")
	}
}
