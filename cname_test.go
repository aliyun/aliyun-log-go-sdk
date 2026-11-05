package sls

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type cnameRoundTripper func(*http.Request) (*http.Response, error)

func (f cnameRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestCnameRequestRouting(t *testing.T) {
	for _, endpoint := range []string{"logs.example.com", "http://logs.example.com:8080", "https://logs.example.com:8443", "http://127.0.0.1:8080", "http://[::1]:8080"} {
		for _, enabled := range []bool{false, true} {
			if strings.Contains(endpoint, "[::1]") && !enabled {
				continue
			}
			for _, project := range []string{"my-project", ""} {
				for _, auth := range []AuthVersionType{AuthV1, AuthV4} {
					for _, path := range []string{"client", "converted", "legacy", "provider"} {
						t.Run(fmt.Sprintf("%s/%t/%s/%s/%s", endpoint, enabled, project, auth, path), func(t *testing.T) {
							var captured *http.Request
							httpClient := &http.Client{Transport: cnameRoundTripper(func(r *http.Request) (*http.Response, error) {
								captured = r
								return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}, nil
							})}
							client := CreateNormalInterfaceV2(endpoint, NewStaticCredentialsProvider("test-id", "test-secret", "test-token")).(*Client)
							client.SetIsCname(enabled)
							client.SetHTTPClient(httpClient)
							client.SetRegion("cn-hangzhou")
							client.SetAuthVersion(auth)
							client.CommonHeaders = map[string]string{HTTPHeaderProject: "wrong-project"}
							client.InnerHeaders = map[string]string{HTTPHeaderProject: "wrong-project"}
							if !enabled || project == "" {
								client.CommonHeaders = nil
								client.InnerHeaders = nil
							}
							headers := map[string]string{HTTPHeaderBodyRawSize: "0"}
							const uri = "/logstores?offset=0&size=10"
							var resp *http.Response
							var err error
							if path == "client" {
								resp, err = client.request(project, http.MethodGet, uri, headers, nil)
							} else {
								var p *LogProject
								switch path {
								case "converted":
									p = convert(client, project)
								case "legacy":
									p, err = NewLogProject(project, endpoint, "test-id", "test-secret")
								case "provider":
									p, err = NewLogProjectV2(project, endpoint, client.credentialsProvider)
								}
								if err != nil {
									t.Fatal(err)
								}
								p.IsCname = enabled
								p.AuthVersion, p.Region = auth, client.Region
								p.httpClient = httpClient
								resp, err = p.RawRequest(http.MethodGet, uri, headers, nil)
							}
							if err != nil {
								t.Fatal(err)
							}
							defer resp.Body.Close()
							if captured == nil {
								t.Fatal("request was not sent")
							}
							wantHost := strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
							if !enabled && project != "" {
								wantHost = project + "." + wantHost
							}
							if captured.URL.Host != wantHost || captured.Host != wantHost {
								t.Fatalf("host: URL=%q request=%q want=%q", captured.URL.Host, captured.Host, wantHost)
							}
							wantScheme := "http"
							if strings.HasPrefix(endpoint, "https://") {
								wantScheme = "https"
							}
							if captured.URL.Scheme != wantScheme {
								t.Fatalf("scheme: %s", captured.URL.Scheme)
							}
							wantProject := ""
							if enabled {
								wantProject = project
							}
							if got := captured.Header.Get(HTTPHeaderProject); got != wantProject {
								t.Fatalf("project header: got %q, want %q", got, wantProject)
							}
							if wantProject == "" {
								if _, ok := captured.Header[http.CanonicalHeaderKey(HTTPHeaderProject)]; ok {
									t.Fatal("unexpected project header")
								}
							}
							var signer Signer = NewSignerV1("test-id", "test-secret")
							if auth == AuthV4 {
								signer = NewSignerV4("test-id", "test-secret", client.Region)
							}
							original := headers[HTTPHeaderAuthorization]
							delete(headers, HTTPHeaderAuthorization)
							if err := signer.Sign(http.MethodGet, uri, headers, nil); err != nil {
								t.Fatal(err)
							}
							if headers[HTTPHeaderAuthorization] != original {
								t.Fatal("wire headers do not match signature")
							}
							if wantProject != "" {
								delete(headers, HTTPHeaderProject)
								delete(headers, HTTPHeaderAuthorization)
								if err := signer.Sign(http.MethodGet, uri, headers, nil); err != nil {
									t.Fatal(err)
								}
								if headers[HTTPHeaderAuthorization] == original {
									t.Fatal("project header was not signed")
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestCnameProjectCachedURL(t *testing.T) {
	p, err := NewLogProject("project", "https://logs.example.com:8443", "id", "secret")
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false, true} {
		p.SetIsCname(enabled)
		want := "https://project.logs.example.com:8443"
		if enabled {
			want = "https://logs.example.com:8443"
		}
		if got := p.getBaseURL(); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestCnameProjectConcurrentURL(t *testing.T) {
	p, err := NewLogProjectV2("project", "https://logs.example.com", NewStaticCredentialsProvider("id", "secret", ""))
	if err != nil {
		t.Fatal(err)
	}
	p.IsCname = true
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := p.getBaseURL(); got != "https://logs.example.com" {
				t.Errorf("unexpected URL: %s", got)
			}
		}()
	}
	wg.Wait()
}

func TestCnameClientInterface(t *testing.T) {
	for _, client := range []ClientInterface{
		CreateNormalInterface("logs.example.com", "id", "secret", ""),
		CreateNormalInterfaceV2("logs.example.com", NewStaticCredentialsProvider("id", "secret", "")),
	} {
		wrapped := &TokenAutoUpdateClient{logClient: client}
		wrapped.SetIsCname(true)
		if !client.(*Client).IsCname {
			t.Fatal("option was not forwarded")
		}
		wrapped.SetIsCname(false)
		if client.(*Client).IsCname {
			t.Fatal("option was not disabled")
		}
	}
}
