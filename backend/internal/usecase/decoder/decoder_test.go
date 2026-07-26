package decoder

import "testing"

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []struct {
		op    string
		input string
	}{
		{"base64", "hello world"},
		{"base64url", "hello world?"},
		{"base32", "hello world"},
		{"hex", "hello world"},
		{"url", "a b&c=d"},
		{"html", `<script>"quoted"</script>`},
		{"gzip", "hello world hello world hello world"},
	}

	for _, c := range cases {
		t.Run(c.op, func(t *testing.T) {
			encoded, err := Encode(c.op, c.input)
			if err != nil {
				t.Fatalf("Encode(%q) error: %v", c.op, err)
			}
			decoded, err := Decode(c.op, encoded)
			if err != nil {
				t.Fatalf("Decode(%q, %q) error: %v", c.op, encoded, err)
			}
			if decoded != c.input {
				t.Fatalf("round trip = %q, want %q", decoded, c.input)
			}
		})
	}
}

func TestEncode_HashOps(t *testing.T) {
	cases := []struct {
		op     string
		hexLen int
	}{
		{"md5", 32},
		{"sha1", 40},
		{"sha256", 64},
		{"sha512", 128},
	}
	for _, c := range cases {
		t.Run(c.op, func(t *testing.T) {
			out, err := Encode(c.op, "hello world")
			if err != nil {
				t.Fatalf("Encode(%q) error: %v", c.op, err)
			}
			if len(out) != c.hexLen {
				t.Fatalf("Encode(%q) = %q (len %d), want hex length %d", c.op, out, len(out), c.hexLen)
			}
		})
	}
}

func TestDecode_HashOpsAreOneWay(t *testing.T) {
	for _, op := range []string{"md5", "sha1", "sha256", "sha512"} {
		if _, err := Decode(op, "deadbeef"); err == nil {
			t.Fatalf("Decode(%q) should error — hashes are one-way", op)
		}
	}
}

func TestEncodeDecode_UnknownOp(t *testing.T) {
	if _, err := Encode("not-a-real-op", "x"); err == nil {
		t.Fatal("Encode with an unknown op should error")
	}
	if _, err := Decode("not-a-real-op", "x"); err == nil {
		t.Fatal("Decode with an unknown op should error")
	}
}

func TestDecode_InvalidInput(t *testing.T) {
	cases := []struct {
		op    string
		input string
	}{
		{"base64", "not valid base64!!"},
		{"base32", "not valid base32!!"},
		{"hex", "not hex zz"},
		{"url", "%zz"},
		{"gzip", "aGVsbG8="}, // valid base64, but not gzip-compressed data
	}
	for _, c := range cases {
		t.Run(c.op, func(t *testing.T) {
			if _, err := Decode(c.op, c.input); err == nil {
				t.Fatalf("Decode(%q, %q) should error on malformed input", c.op, c.input)
			}
		})
	}
}

func TestSmartDecode_PlainTextDetectsNothing(t *testing.T) {
	steps := SmartDecode("just a plain sentence")
	if len(steps) != 0 {
		t.Fatalf("steps = %+v, want none for plain text", steps)
	}
}

func TestSmartDecode_SingleLayers(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		wantOp string
		want   string
	}{
		{"html entity", "AT&amp;T", "html", "AT&T"},
		{"url percent-encoding", "a%20b%26c", "url", "a b&c"},
		{"hex", "68656c6c6f", "hex", "hello"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			steps := SmartDecode(c.input)
			if len(steps) == 0 {
				t.Fatalf("expected at least one step for %q", c.input)
			}
			if steps[0].Op != c.wantOp {
				t.Fatalf("steps[0].Op = %q, want %q (steps=%+v)", steps[0].Op, c.wantOp, steps)
			}
			if steps[len(steps)-1].Output != c.want {
				t.Fatalf("final output = %q, want %q", steps[len(steps)-1].Output, c.want)
			}
		})
	}
}

func TestSmartDecode_GzipPlusBase64DetectsAsOneGzipStep(t *testing.T) {
	encoded, err := Encode("gzip", "hello world")
	if err != nil {
		t.Fatalf("Encode(gzip) error: %v", err)
	}
	steps := SmartDecode(encoded)
	if len(steps) != 1 {
		t.Fatalf("steps = %+v, want exactly one gzip step (not a stray Base64 step first)", steps)
	}
	if steps[0].Op != "gzip" || steps[0].Output != "hello world" {
		t.Fatalf("steps[0] = %+v, want op=gzip output=%q", steps[0], "hello world")
	}
}

func TestSmartDecode_ChainedLayersPeelOneAtATime(t *testing.T) {
	// base64(hex("abc")) — smart decode should peel base64 first, then hex.
	//
	// The reverse composition, hex(base64(x)), is NOT usable here: standard
	// base64 padding makes any base64 output's length always a multiple of
	// 4, so hex-encoding it always yields a string whose length is a
	// multiple of 8 — which satisfies detect()'s base64 length check too,
	// so the outer hex layer gets permanently misdetected as base64 no
	// matter what x is. That's an inherent ambiguity in the heuristic
	// (hex's charset is a subset of base64's), not something this test
	// should paper over by picking different bytes.
	hexLayer, _ := Encode("hex", "abc")
	outer, _ := Encode("base64", hexLayer)

	steps := SmartDecode(outer)
	if len(steps) != 2 {
		t.Fatalf("steps = %+v, want 2 layers peeled", steps)
	}
	if steps[0].Op != "base64" || steps[1].Op != "hex" {
		t.Fatalf("peel order = [%s, %s], want [base64, hex]", steps[0].Op, steps[1].Op)
	}
	if steps[1].Output != "abc" {
		t.Fatalf("final output = %q, want %q", steps[1].Output, "abc")
	}
}

func TestSmartDecode_StopsAtMaxSteps(t *testing.T) {
	// Wrap "x" in more hex layers than maxSmartSteps allows.
	current := "x"
	for i := 0; i < maxSmartSteps+3; i++ {
		var err error
		current, err = Encode("hex", current)
		if err != nil {
			t.Fatalf("Encode(hex) error: %v", err)
		}
	}

	steps := SmartDecode(current)
	if len(steps) > maxSmartSteps {
		t.Fatalf("len(steps) = %d, want at most %d", len(steps), maxSmartSteps)
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		input  string
		wantOp string
		wantOK bool
	}{
		{"", "", false},
		{"   ", "", false},
		{"just text with spaces", "", false},
		{"AT&amp;T", "html", true},
		{"a%20b", "url", true},
		{"68656c6c6f", "hex", true},
		{"aGVsbG8=", "base64", true},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			op, ok := detect(c.input)
			if ok != c.wantOK {
				t.Fatalf("detect(%q) ok = %v, want %v", c.input, ok, c.wantOK)
			}
			if ok && op != c.wantOp {
				t.Fatalf("detect(%q) op = %q, want %q", c.input, op, c.wantOp)
			}
		})
	}
}
