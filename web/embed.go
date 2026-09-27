// Package web holds the browser renderer. The CLI embeds it and injects a
// scene by replacing ScenePlaceholder.
package web

import _ "embed"

//go:embed medallion.html
var Medallion []byte

// SceneTag opens the script element that carries an embedded scene.
const SceneTag = `<script id="codeviz-scene" type="application/json">`

// ScenePlaceholder is the empty scene slot in Medallion.
const ScenePlaceholder = SceneTag + `null</script>`
