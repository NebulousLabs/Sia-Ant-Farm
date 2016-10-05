package ant

import (
	"os"
	"testing"

	"github.com/NebulousLabs/Sia/api"
)

func TestNewAnt(t *testing.T) {
	config := AntConfig{
		APIAddr:      "localhost:31337",
		RPCAddr:      "localhost:31338",
		HostAddr:     "localhost:31339",
		SiaDirectory: os.TempDir(),
		SiadPath:     "siad",
	}

	_, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	c := api.NewClient("localhost:31337", "")
	if err = c.Get("/consensus", nil); err != nil {
		t.Fatal(err)
	}
}
