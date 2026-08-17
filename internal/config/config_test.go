package config

import (
	"reflect"
	"testing"
)

func TestParseSkipList(t *testing.T) {
	input := `
# System DBs
postgres
  template0  
template1

# Custom DBs to ignore
ignored_db_1
# another comment
ignored_db_2
`
	want := []string{"postgres", "template0", "template1", "ignored_db_1", "ignored_db_2"}
	got := ParseSkipList(input)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseSkipList() = %v, want %v", got, want)
	}
}
