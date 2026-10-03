package material_test

import (
	"testing"

	"github.com/Mercer08572/stock-flow/internal/material/material"
)

func TestCommensurableUnitTypes(t *testing.T) {
	cases := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"same weight", material.UnitTypeNameWeight, material.UnitTypeNameWeight, true},
		{"same count", material.UnitTypeNameCount, material.UnitTypeNameCount, true},
		{"package to count", material.UnitTypeNamePackage, material.UnitTypeNameCount, true},
		{"count to package", material.UnitTypeNameCount, material.UnitTypeNamePackage, true},
		{"weight to count", material.UnitTypeNameWeight, material.UnitTypeNameCount, false},
		{"weight to package", material.UnitTypeNameWeight, material.UnitTypeNamePackage, false},
		{"length to area", material.UnitTypeNameLength, material.UnitTypeNameArea, false},
		{"other to other", material.UnitTypeNameOther, material.UnitTypeNameOther, true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := material.CommensurableUnitTypes(testCase.from, testCase.to); got != testCase.want {
				t.Fatalf("CommensurableUnitTypes(%q, %q) = %v, want %v", testCase.from, testCase.to, got, testCase.want)
			}
		})
	}
}

func TestUnitConversionCheckDirection(t *testing.T) {
	check := material.UnitConversionCheck{
		Status:     material.UnitConversionOK,
		FromUnitID: 30,
		ToUnitID:   20,
	}

	if !check.TouchesBaseUnit(20) {
		t.Fatal("expected the check to touch base unit 20")
	}
	if check.TouchesBaseUnit(99) {
		t.Fatal("expected the check not to touch base unit 99")
	}

	from, to := check.NormalizedDirection(20)
	if from != 20 || to != 30 {
		t.Fatalf("expected normalized direction 20 -> 30, got %d -> %d", from, to)
	}
}
