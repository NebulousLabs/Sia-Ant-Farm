package ant

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewJobRunner(t *testing.T) {
	datadir, err := ioutil.TempDir("", "testing-data")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(datadir)
	siad, err := newSiad("siad", datadir, "localhost:31337", "localhost:31338", "localhost:31339")
	if err != nil {
		t.Fatal(err)
	}
	defer stopSiad("localhost:31337", siad.Process)

	j, err := newJobRunner("localhost:31337", "", datadir)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Stop()
}
