package integration

import (
	jsonMod "encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/khulnasoft/kengine/v2/kenginetest"

	_ "github.com/khulnasoft/kengine/v2/internal/testmocks"
)

func TestKenginefileAdaptToJSON(t *testing.T) {
	// load the list of test files from the dir
	files, err := os.ReadDir("./kenginefile_adapt")
	if err != nil {
		t.Errorf("failed to read kenginefile_adapt dir: %s", err)
	}

	// prep a regexp to fix strings on windows
	winNewlines := regexp.MustCompile(`\r?\n`)

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		// read the test file
		filename := f.Name()
		data, err := os.ReadFile("./kenginefile_adapt/" + filename)
		if err != nil {
			t.Errorf("failed to read %s dir: %s", filename, err)
		}

		// split the Kenginefile (first) and JSON (second) parts
		// (append newline to Kenginefile to match formatter expectations)
		parts := strings.Split(string(data), "----------")
		kenginefile, json := strings.TrimSpace(parts[0])+"\n", strings.TrimSpace(parts[1])

		// replace windows newlines in the json with unix newlines
		json = winNewlines.ReplaceAllString(json, "\n")

		// replace os-specific default path for file_server's hide field
		replacePath, _ := jsonMod.Marshal(fmt.Sprint(".", string(filepath.Separator), "Kenginefile"))
		json = strings.ReplaceAll(json, `"./Kenginefile"`, string(replacePath))

		// run the test
		ok := kenginetest.CompareAdapt(t, filename, kenginefile, "kenginefile", json)
		if !ok {
			t.Errorf("failed to adapt %s", filename)
		}
	}
}
