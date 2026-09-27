// Package htmlpage is the Renderer adapter that writes a standalone HTML
// page: the embedded medallion renderer with the scene injected.
package htmlpage

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/arsmn/codeviz/internal/adapters/scene"
	"github.com/arsmn/codeviz/internal/domain"
	"github.com/arsmn/codeviz/web"
)

type Page struct{}

func (Page) Render(w io.Writer, a domain.Analysis) error {
	// json.Marshal escapes <, > and &, so the data cannot close the script tag.
	data, err := json.Marshal(scene.From(a))
	if err != nil {
		return err
	}
	tpl := web.Medallion
	i := bytes.Index(tpl, []byte(web.ScenePlaceholder))
	if i < 0 {
		return errors.New("renderer template has no scene placeholder")
	}
	for _, part := range [][]byte{tpl[:i], []byte(web.SceneTag), data, []byte("</script>"), tpl[i+len(web.ScenePlaceholder):]} {
		if _, err := w.Write(part); err != nil {
			return err
		}
	}
	return nil
}
