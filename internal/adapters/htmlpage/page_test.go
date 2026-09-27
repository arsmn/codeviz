package htmlpage

import (
	"bytes"
	"strings"
	"testing"

	"github.com/arsmn/codeviz/internal/domain"
	"github.com/arsmn/codeviz/web"
)

func TestRenderInjectsScene(t *testing.T) {
	a := domain.Analysis{Name: "</script><b>", Language: "go", Packages: []domain.PackageAnalysis{{ID: "p"}}}
	var buf bytes.Buffer
	if err := (Page{}).Render(&buf, a); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, web.ScenePlaceholder) {
		t.Error("placeholder was not replaced")
	}
	if !strings.Contains(out, web.SceneTag+`{"schema":"codeviz.scene/v0"`) {
		t.Error("scene JSON not injected after the scene tag")
	}
	if strings.Contains(out, "</script><b>") {
		t.Error("project name was not escaped inside the script element")
	}
}
