# Middleware Security Audit

**Date:** 2026-04-08
**Scope:** `internal/middleware/` subpackage
**Auditor:** Claude (automated static analysis)

---

## Summary

| Severity | Count |
| -------- | ----- |
| Critical | 0     |
| High     | 0     |
| Medium   | 0     |
| Low      | 0     |
| Info     | 0     |

*Remediation applied 2026-05-06; issues below are documented as closed.*

---

## Findings

### Critical

#### 1. Decompression bomb (compression.go:152-169)

`DecompressionMiddleware` decompresses gzip request bodies without any size limit. An attacker can send a small gzip-compressed payload that decompresses to gigabytes of data, exhausting server memory and causing denial of service.

If `RequestSizeLimitMiddleware` runs before decompression, the limit applies to the compressed payload (small), so the bomb still passes. If it runs after, the body is already fully decompressed in memory.

**Recommendation:** Wrap the gzip reader with `io.LimitReader` directly inside `DecompressionMiddleware` to cap the decompressed output size.

**Status:** Resolved. `DecompressionMiddlewareWithConfig` now wraps the gzip reader with `io.LimitReader` using a default `MaxDecompressedSize` of 1MB.

---

### High

#### 2. CSRF bypass for all `/api/` paths (csrf.go:74-77)

The CSRF middleware unconditionally skips validation for any request whose path starts with `/api/`. If API endpoints accept cookie-based authentication (which is common when `AllowCredentials` is true in CORS), they are vulnerable to cross-site request forgery.

**Recommendation:** Either require CSRF tokens for cookie-authenticated API endpoints, or ensure API endpoints exclusively use Bearer token authentication (inherently CSRF-resistant). Document the security assumption explicitly.

**Status:** Resolved. `CSRFMiddleware` now only skips validation for `/api/` requests when a `Bearer` token is present in the `Authorization` header. Cookie-authenticated API requests will still require CSRF validation.

#### 3. Race condition in suspicious activity detection (security_logging.go:223-237)

The `requestCounts` map and `lastReset` variable are package-level globals accessed without any synchronization. Under concurrent requests, this causes a data race: concurrent reads and writes to a Go map can crash the program or produce incorrect counts, allowing bypass of the rapid-request detection.

**Recommendation:** Protect `requestCounts` and `lastReset` with a `sync.Mutex`, or replace with a proper concurrent-safe rate limiter.

**Status:** Resolved. `requestCounts` and `lastReset` were moved into a `rapidRequestTracker` struct protected by `sync.Mutex`, eliminating the data race.

#### 4. Fragile CORS origin configuration (cors.go:30-46)

In production/staging, the origin configuration is split across two separate `os.Getenv` calls with confusing fallthrough logic. When `CORS_ALLOWED_ORIGINS` is empty, `AllowedOrigins` defaults to an empty slice (rejecting all cross-origin requests). The logic is fragile and a misconfiguration could either block all legitimate traffic or open origins too broadly.

**Recommendation:** Consolidate the origin-reading logic to a single place. Add explicit validation that rejects `*` when `AllowCredentials` is true. Fail loudly on misconfiguration.

**Status:** Resolved. Origins are read and trimmed in one code path; wildcard with credentials is rejected (`panic`).

---

### Medium

#### 5. Wildcard origin reflects attacker origin (cors.go:66-76)

When `AllowedOrigins` contains `"*"` and `AllowCredentials` is true, the code reflects the specific requesting origin as `Access-Control-Allow-Origin`. While the `Access-Control-Allow-Credentials` header is correctly suppressed, reflecting arbitrary origins is functionally equivalent to a wildcard and can be exploitable if cookie-authenticated endpoints exist.

**Recommendation:** When `AllowedOrigins` contains `"*"` and `AllowCredentials` is true, reject the configuration at startup or refuse to reflect arbitrary origins.

**Status:** Resolved (same validation as #4).

#### 6. Unbounded in-memory cache (cache.go:57-61)

`InMemoryCache.Set` does not enforce `CacheConfig.MaxSize`. The `MaxSize` field is defined but never checked. An attacker can cause unbounded memory growth by requesting many unique cacheable URLs, eventually causing an out-of-memory condition.

**Recommendation:** Enforce `MaxSize` in the `Set` method. Implement LRU eviction when the limit is reached.

**Status:** Resolved. LRU eviction enforced when entry count exceeds `MaxSize`.

#### 7. Cache invalidation clears entire cache (cache.go:201-213)

`CacheInvalidationMiddleware` calls `cache.Clear()` on every POST, PUT, or DELETE request. An attacker can repeatedly send mutation requests to any endpoint to constantly clear the cache, negating all caching benefits and increasing backend load.

**Recommendation:** Implement targeted cache invalidation based on the affected resource path rather than clearing the entire cache.

**Status:** Resolved. Invalidation clears keys indexed under `CacheConfig.CacheablePaths` prefixes that match the mutation path; non-indexed entries are untouched.

#### 8. MD5 used for cache key hashing (cache.go:197)

MD5 is used to hash cache keys. MD5 collisions are trivially constructible, enabling an attacker to craft two different URLs that produce the same cache key, causing cache poisoning (one user's response served to another).

**Recommendation:** Use SHA-256 or use the raw key string directly.

**Status:** Resolved. Keys use SHA-256 hex.

#### 9. Bypassable SQL/XSS detection (security_logging.go:161-198)

`containsSQLInjection` and `containsXSS` use simple string matching with a tiny set of hardcoded patterns. These are trivially bypassed with URL encoding, double encoding, Unicode case variations, or SQL comment injection (`UN/**/ION SEL/**/ECT`). The middleware only logs and never blocks, providing a false sense of security.

**Recommendation:** If this is intended as detection, URL-decode input before checking patterns and document that it is informational only. If intended as a WAF, use a proper pattern library.

**Status:** Resolved (URL-aware decoding before checks). Detection remains informational only—not a WAF replacement.

#### 10. Swagger routes weaken CSP (security_headers.go:39-40)

The CSP for Swagger routes includes `'unsafe-inline'` and `'unsafe-eval'` for scripts. `unsafe-eval` significantly weakens XSS protections. If an XSS vulnerability exists in Swagger UI (which has had CVEs), this policy permits exploitation.

**Recommendation:** Investigate nonce-based CSP for Swagger UI. Ensure Swagger routes are not exposed in production, or restrict access to authenticated/internal users.

**Status:** Resolved. Swagger is not mounted when `GOYCO_ENV` is `production` unless `SWAGGER_ENABLED=true`. Relaxed Swagger CSP applies only when swagger is exposed; otherwise strict CSP is used even for `/swagger` paths.

#### 11. Silent CSP nonce degradation (security_headers.go:53-56)

If `GenerateCSPNonce()` fails, the error is silently swallowed and the nonce is set to an empty string. The fallback CSP uses `'self'` without a nonce, which is less secure.

**Recommendation:** Log the error. Consider returning a 500 error instead of silently degrading security.

**Status:** Resolved. Failures are logged via the standard logger.

---

### Low

#### 12. Ambiguous unauthenticated user ID (auth.go:76-81)

`GetUserIDFromContext` returns `0` for unauthenticated users. If user ID 0 is ever valid in the database, or if downstream code does not check for the zero value, this could lead to authorization issues.

**Recommendation:** Use a pointer type (`*uint`) or a dedicated error/boolean return to distinguish "no user" from "user with ID 0".

**Status:** Resolved. `GetUserIDFromContext` returns `*uint` (`nil` when absent).

#### 13. CSRF cookie blocks double-submit via JS (csrf.go:31)

The CSRF token cookie is set with `HttpOnly: true`. Client-side JavaScript cannot read the cookie to include the token in `X-CSRF-Token` headers. The double-submit cookie pattern typically requires `HttpOnly: false` for AJAX compatibility.

**Recommendation:** If AJAX-based CSRF submission is intended, set `HttpOnly: false` on the CSRF cookie (safe because CSRF tokens are not session credentials). Alternatively, provide a separate endpoint to fetch the token.

**Status:** Resolved. CSRF cookie uses `HttpOnly: false`.

#### 14. Deprecated X-XSS-Protection header (security_headers.go:34)

`X-XSS-Protection: 1; mode=block` is deprecated and removed from modern browsers. In some older browsers, it can actually introduce XSS vulnerabilities. CSP is the proper replacement.

**Recommendation:** Remove the header or set it to `0` to disable the legacy XSS auditor.

**Status:** Resolved. Header is no longer sent.

---

### Info

#### 15. HSTS relies on client-spoofable headers (security_headers.go:108-111)

When `TrustProxyHeaders` is true, HSTS is set based on `X-Forwarded-Proto`, which is client-controllable if the proxy does not strip/overwrite it. The `TrustProxyHeaders` guard is the right approach, but the requirement for a properly configured reverse proxy should be documented.

**Status:** Documented in README (reverse proxy / HSTS section).

#### 16. Background cache operations discard errors (cache.go:144-146, 205-207)

Cache `Set` and `Clear` operations run in goroutines with errors silently discarded. If using a remote cache implementation, failures would go unnoticed.

**Recommendation:** Log errors from background cache operations.

**Status:** Resolved. Background `Set` errors are logged.

#### 17. Middleware ordering dependency (compression.go + request_size.go)

The effectiveness of `RequestSizeLimitMiddleware` depends on its position relative to `DecompressionMiddleware`. Neither middleware documents or enforces the required ordering.

**Recommendation:** Document the required middleware ordering. Ideally, apply the size limit inside `DecompressionMiddleware` itself.

**Status:** Documented in README (#1 also limits decompressed size in middleware).

---

## Roadmap

### Phase 1 -- Critical and crash-inducing fixes

These issues can cause service outages and should be addressed first.

- [x] **Add `io.LimitReader` to `DecompressionMiddleware`** to prevent decompression bombs (#1, #17) — Fixed 2026-04-23
- [x] **Add `sync.Mutex` to `security_logging.go`** to prevent race condition crashes (#3) — Fixed 2026-04-23

### Phase 2 -- Authentication and cross-origin hardening

These issues can be exploited by attackers to bypass security controls.

- [x] **Audit `/api/` routes for cookie auth usage** and either require CSRF tokens or confirm Bearer-only auth (#2) — Fixed 2026-04-23
- [x] **Consolidate and validate CORS origin configuration** with startup-time checks (#4, #5) — Fixed 2026-05-06
- [x] **Set CSRF cookie `HttpOnly: false`** if double-submit via JS headers is intended (#13) — Fixed 2026-05-06

### Phase 3 -- Resource exhaustion and cache integrity

These issues enable denial-of-service or data integrity attacks.

- [x] **Enforce `MaxSize` in `InMemoryCache.Set`** with LRU eviction (#6) — Fixed 2026-05-06
- [x] **Replace blanket `cache.Clear()` with targeted invalidation** (#7) — Fixed 2026-05-06
- [x] **Replace MD5 with SHA-256** for cache key hashing (#8) — Fixed 2026-05-06

### Phase 4 -- Detection, headers, and hardening

Lower-priority improvements to defense-in-depth measures.

- [x] **URL-decode input before SQL/XSS pattern matching** in `security_logging.go` (#9) — Fixed 2026-05-06
- [x] **Restrict Swagger routes** to non-production or authenticated access; investigate nonce-based CSP (#10) — Fixed 2026-05-06 (exposure gated; nonce CSP optional follow-up)
- [x] **Log CSP nonce generation failures** instead of silent degradation (#11) — Fixed 2026-05-06
- [x] **Remove or disable `X-XSS-Protection`** header (#14) — Fixed 2026-05-06
- [x] **Return `*uint` from `GetUserIDFromContext`** to avoid zero-value ambiguity (#12) — Fixed 2026-05-06
- [x] **Log background cache operation errors** (#16) — Fixed 2026-05-06
- [x] **Document HSTS proxy requirements** and middleware ordering (#15, #17) — Fixed 2026-05-06
