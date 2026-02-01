package eval

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveAndLoadGold(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	ann := &GoldAnnotation{
		DocID:      "test-doc",
		Unit:       "block",
		Boundaries: []int{5, 2, 8}, // intentionally unsorted
		Params: &GoldParams{
			EvalStage:     "raw",
			Tolerance:     intPtr(0),
			MinGap:        intPtr(2),
			PseudoHeading: boolPtr(false),
		},
		Notes: map[string]string{"author": "test"},
	}

	if err := SaveGold(path, ann); err != nil {
		t.Fatalf("SaveGold: %v", err)
	}

	loaded, err := LoadGold(path)
	if err != nil {
		t.Fatalf("LoadGold: %v", err)
	}

	// Boundaries should be sorted after save/load
	want := []int{2, 5, 8}
	if !reflect.DeepEqual(loaded.Boundaries, want) {
		t.Errorf("Boundaries: want %v, got %v", want, loaded.Boundaries)
	}
	if loaded.DocID != "test-doc" {
		t.Errorf("DocID: want test-doc, got %s", loaded.DocID)
	}
	if loaded.Unit != "block" {
		t.Errorf("Unit: want block, got %s", loaded.Unit)
	}
	if loaded.Params == nil || loaded.Params.EvalStage != "raw" || loaded.Params.Tolerance == nil || *loaded.Params.Tolerance != 0 {
		t.Errorf("Params: unexpected values %+v", loaded.Params)
	}
	if loaded.Notes["author"] != "test" {
		t.Errorf("Notes[author]: want test, got %s", loaded.Notes["author"])
	}
}

func TestLoadGold_FileNotFound(t *testing.T) {
	_, err := LoadGold("/nonexistent/path.json")
	if err == nil {
		t.Error("LoadGold should fail for nonexistent file")
	}
}

func TestLoadGold_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadGold(path)
	if err == nil {
		t.Error("LoadGold should fail for invalid JSON")
	}
}

func TestSaveGold_NilNotes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	ann := &GoldAnnotation{
		DocID:      "no-notes",
		Unit:       "block",
		Boundaries: []int{1},
	}

	if err := SaveGold(path, ann); err != nil {
		t.Fatalf("SaveGold: %v", err)
	}

	loaded, err := LoadGold(path)
	if err != nil {
		t.Fatalf("LoadGold: %v", err)
	}
	if loaded.Notes != nil {
		t.Errorf("Notes should be nil when omitempty, got %v", loaded.Notes)
	}
}

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
