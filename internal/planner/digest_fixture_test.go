package planner

import "testing"

func TestFixtureTreeDigest(t *testing.T) {
	digest, err := sourceTreeDigest("../../testdata/ledger-v0.31")
	if err != nil {
		t.Fatal(err)
	}
	if want := "sha256:91af005372ea71ecceb5908135536c27603cad0ed9826815e24f8b65b840382c"; digest != want {
		t.Fatalf("fixture digest = %s, want %s", digest, want)
	}
}
