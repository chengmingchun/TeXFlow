package main

import "testing"

func TestParseSyncTeXView(t *testing.T) {
	output := `SyncTeX result begin
Output:C:\resume\main.pdf
Page:2
x:34.478020
y:77.873955
h:25.511646
v:78.819611
W:544.252319
H:8.335913
before:
SyncTeX result end`
	point, err := parseSyncTeXView(output)
	if err != nil {
		t.Fatal(err)
	}
	if point.page != 2 || point.h != 25.511646 || point.v != 78.819611 || point.height != 8.335913 {
		t.Fatalf("unexpected point: %#v", point)
	}
}

func TestParseSyncTeXViewRequiresCoordinates(t *testing.T) {
	if _, err := parseSyncTeXView("SyncTeX Warning: No tag"); err == nil {
		t.Fatal("expected missing coordinates to fail")
	}
}
