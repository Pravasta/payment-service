package middleware

import "net/http"

// StripTrailingSlash menghapus trailing slash dari path (mis. "/readyz/" ->
// "/readyz") sebelum routing, sehingga tidak menghasilkan 404. Root "/" dibiarkan.
func StripTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := r.URL.Path; len(p) > 1 && p[len(p)-1] == '/' {
			r.URL.Path = p[:len(p)-1]
		}
		next.ServeHTTP(w, r)
	})
}
