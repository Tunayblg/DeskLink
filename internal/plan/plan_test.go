package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddToggleDeleteReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.Add(" Daily ", "", "")
	if err != nil {
		t.Fatal(err)
	}
	id := list[0].ID
	list, err = s.Toggle(id, "2026-11-08")
	if err != nil || list[0].DoneOn != "2026-11-08" {
		t.Fatalf("toggle: %+v %v", list, err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.List()[0].DoneOn != "2026-11-08" {
		t.Fatal("completion not persisted")
	}
	list, err = s.Toggle(id, "2026-11-09")
	if err != nil || list[0].DoneOn != "2026-11-09" {
		t.Fatalf("daily next day: %+v %v", list, err)
	}
	list, _ = s.Toggle(id, "2026-11-09")
	if list[0].DoneOn != "" {
		t.Fatal("undo")
	}
	list, _ = s.Add("Dated", "09:00", "2026-11-01")
	dated := list[1].ID
	list, err = s.Toggle(dated, "2026-11-08")
	if err != nil || list[1].DoneOn == "" {
		t.Fatal("dated completion", err)
	}
	list, _ = s.Toggle(dated, "2026-11-09")
	if list[1].DoneOn != "" {
		t.Fatal("dated completion must undo even on later day")
	}
	list, err = s.Delete(id)
	if err != nil || len(list) != 1 {
		t.Fatal("delete", err)
	}
	list, err = s.Delete("unknown")
	if err != nil || len(list) != 1 {
		t.Fatal("unknown delete", err)
	}
}

func TestValidation(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "plan.json"))
	for _, b := range [][3]string{{"", "", ""}, {strings.Repeat("ş", 61), "", ""}, {"x", "9:00", ""}, {"x", "24:00", ""}, {"x", "12:60", ""}, {"x", "", "2026-2-01"}, {"x", "", "2026-02-30"}} {
		if _, err := s.Add(b[0], b[1], b[2]); err == nil {
			t.Errorf("accepted %q", b)
		}
	}
	for _, date := range []string{"", "2026-2-01", "2026-02-30"} {
		if _, err := s.Toggle("unknown", date); err == nil {
			t.Errorf("accepted date %q", date)
		}
	}
	if _, err := s.Toggle("unknown", "2026-11-08"); err == nil {
		t.Fatal("unknown toggle")
	}
}

func TestLegacyEndAndOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(`[{"id":"old","title":"Legacy","time":"09:00","end":"invalid","date":"2020-01-01"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Add("Untimed", "", "")
	s.Add("Later", "11:00", "")
	list, err := s.Add("Earlier", "08:00", "")
	if err != nil || len(list) != 4 || list[0].Title != "Earlier" || list[2].Title != "Untimed" {
		t.Fatalf("list: %+v %v", list, err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "end") {
		t.Fatal("legacy end persisted")
	}
}

func TestSaveFailureRollsBack(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "plan.json"))
	list, _ := s.Add("Task", "", "")
	id := list[0].ID
	s.path = t.TempDir() // cannot replace a directory with the temporary file
	if _, err := s.Toggle(id, "2026-11-08"); err == nil {
		t.Fatal("expected save failure")
	}
	if s.List()[0].DoneOn != "" {
		t.Fatal("toggle did not roll back")
	}
	if _, err := s.Delete(id); err == nil {
		t.Fatal("expected delete failure")
	}
	if _, err := s.Add("Other", "", ""); err == nil {
		t.Fatal("expected add failure")
	}
	if len(s.List()) != 1 {
		t.Fatal("mutation did not roll back")
	}
}

func TestTimeOnlyReminder(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "plan.json"))
	list, err := s.Add("", "18:00", "")
	if err != nil || len(list) != 1 || list[0].Title != "" || list[0].Time != "18:00" {
		t.Fatalf("a reminder with only a time should be accepted: %+v %v", list, err)
	}
}
