package openapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yaoapp/yao/config"
	"github.com/yaoapp/yao/openapi"
	"github.com/yaoapp/yao/openapi/signin"
	"github.com/yaoapp/yao/openapi/tests/testutils"
)

func TestSigninLoad(t *testing.T) {
	// Initialize test environment
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	_ = serverURL // Server URL not needed for this test

	// Test loading signin configurations
	err := signin.Load(config.Conf)
	assert.NoError(t, err, "signin.Load should succeed")

	// Test that we can get available languages
	languages := signin.GetAvailableLanguages()
	assert.IsType(t, []string{}, languages, "Should return string slice")
	t.Logf("Available languages: %v", languages)

	// Test default language
	defaultLang := signin.GetDefaultLanguage()
	assert.IsType(t, "", defaultLang, "Should return string")
	t.Logf("Default language: %s", defaultLang)
}

func TestSigninGetConfigs(t *testing.T) {
	// Initialize test environment
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	_ = serverURL // Server URL not needed for this test

	// Load signin configurations
	err := signin.Load(config.Conf)
	assert.NoError(t, err, "signin.Load should succeed")

	// Test getting configs for different languages
	testCases := []string{"", "en", "zh-cn", "fr"}

	for _, lang := range testCases {
		t.Run("lang_"+lang, func(t *testing.T) {
			fullConfig := signin.GetFullConfig(lang)
			publicConfig := signin.GetPublicConfig(lang)

			if fullConfig != nil {
				t.Logf("Full config for '%s': %+v", lang, fullConfig.Title)
				assert.NotNil(t, publicConfig, "Public config should exist if full config exists")

				// Test that public config removes sensitive data from OAuth providers
				if fullConfig.ThirdParty != nil && fullConfig.ThirdParty.Providers != nil {
					for i := range fullConfig.ThirdParty.Providers {
						if publicConfig.ThirdParty != nil && i < len(publicConfig.ThirdParty.Providers) {
							publicProvider := publicConfig.ThirdParty.Providers[i]

							// Check that sensitive OAuth fields are removed
							assert.Empty(t, publicProvider.ClientID, "Client ID should be empty in public config")
							assert.Empty(t, publicProvider.ClientSecret, "Client secret should be empty in public config")
							assert.Nil(t, publicProvider.ClientSecretGenerator, "Client secret generator should be nil in public config")
							assert.Empty(t, publicProvider.Scopes, "Scopes should be empty in public config")
							assert.Nil(t, publicProvider.Endpoints, "Endpoints should be nil in public config")
							assert.Empty(t, publicProvider.Mapping, "Mapping should be empty in public config")

							// Check that display fields are preserved
							assert.NotEmpty(t, publicProvider.ID, "Provider ID should be preserved in public config")
							assert.NotEmpty(t, publicProvider.Title, "Provider title should be preserved in public config")
							// Logo, Color, TextColor might be empty depending on config, so we don't assert NotEmpty
						}
					}
				}

				// Test that public config removes sensitive data from captcha configuration
				if fullConfig.Form != nil && fullConfig.Form.Captcha != nil && fullConfig.Form.Captcha.Options != nil {
					if publicConfig.Form != nil && publicConfig.Form.Captcha != nil && publicConfig.Form.Captcha.Options != nil {
						// Check that secret field is removed
						_, hasSecret := publicConfig.Form.Captcha.Options["secret"]
						assert.False(t, hasSecret, "Captcha secret should be removed from public config")

						// Check that safe fields are preserved (if they exist in full config)
						if _, hasSitekey := fullConfig.Form.Captcha.Options["sitekey"]; hasSitekey {
							_, publicHasSitekey := publicConfig.Form.Captcha.Options["sitekey"]
							assert.True(t, publicHasSitekey, "Captcha sitekey should be preserved in public config")
						}
					}
				}
			} else {
				t.Logf("No config found for language: %s", lang)
			}
		})
	}
}

func TestSigninLanguageNormalization(t *testing.T) {
	// Initialize test environment
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	_ = serverURL // Server URL not needed for this test

	// Load signin configurations
	err := signin.Load(config.Conf)
	assert.NoError(t, err, "signin.Load should succeed")

	// Test that language codes are normalized to lowercase
	config1 := signin.GetFullConfig("EN")
	config2 := signin.GetFullConfig("en")
	assert.Equal(t, config1, config2, "Language codes should be normalized to lowercase")

	config3 := signin.GetPublicConfig("ZH-CN")
	config4 := signin.GetPublicConfig("zh-cn")
	assert.Equal(t, config3, config4, "Language codes should be normalized to lowercase")
}

func TestSigninConfigStructure(t *testing.T) {
	// Initialize test environment
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	_ = serverURL // Server URL not needed for this test

	// Load signin configurations
	err := signin.Load(config.Conf)
	assert.NoError(t, err, "signin.Load should succeed")

	// Get a config to test structure
	config := signin.GetFullConfig("")
	if config != nil {
		t.Logf("Config loaded successfully with title: %s", config.Title)

		// Verify config structure is valid
		assert.IsType(t, &signin.Config{}, config, "Should return correct config type")

		// Test form configuration
		if config.Form != nil {
			t.Logf("Form configuration found")
			if config.Form.Username != nil {
				assert.IsType(t, []string{}, config.Form.Username.Fields, "Username fields should be string slice")
			}
			if config.Form.Captcha != nil {
				assert.IsType(t, map[string]interface{}{}, config.Form.Captcha.Options, "Captcha options should be map")
			}
		}

		// Test third party configuration
		if config.ThirdParty != nil {
			t.Logf("Third party configuration found with %d providers", len(config.ThirdParty.Providers))
			if config.ThirdParty.Providers != nil {
				assert.IsType(t, []*signin.Provider{}, config.ThirdParty.Providers, "Providers should be slice of Provider pointers")
				for i, provider := range config.ThirdParty.Providers {
					t.Logf("Provider %d: %s", i, provider.ID)
					assert.IsType(t, []string{}, provider.Scopes, "Provider scopes should be string slice")
					assert.IsType(t, map[string]string{}, provider.Mapping, "Provider mapping should be string map")
				}
			}
		}
	} else {
		t.Log("No signin configuration found")
	}
}

func TestSigninAPI(t *testing.T) {
	// Initialize test environment
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	// Get base URL from server config
	baseURL := ""
	if openapi.Server != nil && openapi.Server.Config != nil {
		baseURL = openapi.Server.Config.BaseURL
	}

	// Test API endpoints
	testCases := []struct {
		name       string
		endpoint   string
		expectCode int
	}{
		{"get config without locale", "/signin", 200},
		{"get config with en locale", "/signin?locale=en", 200},
		{"get config with zh-cn locale", "/signin?locale=zh-cn", 200},
		{"get config with invalid locale", "/signin?locale=invalid", 200}, // should fallback to default
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := serverURL + baseURL + tc.endpoint
			resp, err := http.Get(url)
			assert.NoError(t, err, "HTTP request should succeed")

			if resp != nil {
				defer resp.Body.Close()
				assert.Equal(t, tc.expectCode, resp.StatusCode, "Expected status code %d", tc.expectCode)

				if resp.StatusCode == 200 {
					// Parse response body
					body, err := io.ReadAll(resp.Body)
					assert.NoError(t, err, "Should read response body")

					var config signin.Config
					err = json.Unmarshal(body, &config)
					assert.NoError(t, err, "Should parse JSON response")

					t.Logf("API response for %s: %s", tc.endpoint, config.Title)

					// Verify it's public config (no sensitive data)
					if config.ThirdParty != nil && config.ThirdParty.Providers != nil {
						for _, provider := range config.ThirdParty.Providers {
							// Check that sensitive OAuth fields are removed from API response
							assert.Empty(t, provider.ClientID, "Client ID should be empty in API response")
							assert.Empty(t, provider.ClientSecret, "Client secret should be empty in API response")
							assert.Nil(t, provider.ClientSecretGenerator, "Client secret generator should be nil in API response")
							assert.Empty(t, provider.Scopes, "Scopes should be empty in API response")
							assert.Nil(t, provider.Endpoints, "Endpoints should be nil in API response")
							assert.Empty(t, provider.Mapping, "Mapping should be empty in API response")

							// Check that display fields are preserved in API response
							assert.NotEmpty(t, provider.ID, "Provider ID should be preserved in API response")
							assert.NotEmpty(t, provider.Title, "Provider title should be preserved in API response")
						}
					}

					// Verify captcha sensitive data is removed from API response
					if config.Form != nil && config.Form.Captcha != nil && config.Form.Captcha.Options != nil {
						_, hasSecret := config.Form.Captcha.Options["secret"]
						assert.False(t, hasSecret, "Captcha secret should be removed from API response")
					}
				}
			}
		})
	}
}

func TestSigninOAuthAuthorizationURL(t *testing.T) {
	// Initialize test environment
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	// Get base URL from server config
	baseURL := ""
	if openapi.Server != nil && openapi.Server.Config != nil {
		baseURL = openapi.Server.Config.BaseURL
	}

	// Test OAuth authorization URL endpoints
	// Note: These should return 500 because OAuth client credentials (CLIENT_ID, etc.)
	// are not set in the test environment, making the provider configuration incomplete.
	// This is the expected secure behavior.
	testCases := []struct {
		name           string
		provider       string
		query          string
		expectCode     int
		expectErrorMsg string
	}{
		{"get google oauth url", "google", "", 500, "Provider configuration is incomplete"},
		{"get microsoft oauth url", "microsoft", "", 500, "Provider configuration is incomplete"},
		{"get apple oauth url", "apple", "", 500, "Provider configuration is incomplete"},
		{"get github oauth url", "github", "", 500, "Provider configuration is incomplete"},
		{"get oauth url with locale", "google", "?locale=en", 500, "Provider configuration is incomplete"},
		{"get oauth url with redirect_uri", "google", "?redirect_uri=https://example.com/callback", 500, "Provider configuration is incomplete"},
		{"get oauth url with state", "google", "?state=test-state-123", 500, "Provider configuration is incomplete"},
		{"get oauth url for nonexistent provider", "nonexistent", "", 404, "OAuth provider 'nonexistent' not found"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := serverURL + baseURL + "/signin/oauth/" + tc.provider + "/authorize" + tc.query
			resp, err := http.Get(url)
			assert.NoError(t, err, "HTTP request should succeed")

			if resp != nil {
				defer resp.Body.Close()
				assert.Equal(t, tc.expectCode, resp.StatusCode, "Expected status code %d", tc.expectCode)

				// Parse response body
				body, err := io.ReadAll(resp.Body)
				assert.NoError(t, err, "Should read response body")

				t.Logf("Response for %s: status=%d, body=%s", tc.provider, resp.StatusCode, string(body))

				// Parse error response to verify the error message
				var errorResponse map[string]interface{}
				err = json.Unmarshal(body, &errorResponse)
				assert.NoError(t, err, "Should parse JSON error response")

				// Verify error message matches expected
				if errorDescription, hasError := errorResponse["error_description"]; hasError {
					errorDescStr, ok := errorDescription.(string)
					assert.True(t, ok, "error_description should be string")
					assert.Equal(t, tc.expectErrorMsg, errorDescStr, "Error message should match expected")
				} else {
					t.Errorf("Response should contain error_description field")
				}

				// Verify error code is present
				if errorCode, hasErrorCode := errorResponse["error"]; hasErrorCode {
					assert.Equal(t, "invalid_request", errorCode, "Error code should be invalid_request")
				} else {
					t.Errorf("Response should contain error field")
				}
			}
		})
	}
}

func TestSigninENVVariableReplacement(t *testing.T) {
	// Initialize test environment
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	_ = serverURL // Server URL not needed for this test

	// Load signin configurations to trigger ENV variable processing
	err := signin.Load(config.Conf)
	assert.NoError(t, err, "signin.Load should succeed")

	// Get full config to check ENV variable replacement
	fullConfig := signin.GetFullConfig("")
	assert.NotNil(t, fullConfig, "Should have a signin configuration")

	// Test captcha ENV variable replacement
	if fullConfig.Form != nil && fullConfig.Form.Captcha != nil && fullConfig.Form.Captcha.Options != nil {
		if sitekey, hasSitekey := fullConfig.Form.Captcha.Options["sitekey"]; hasSitekey {
			sitekeyStr, ok := sitekey.(string)
			assert.True(t, ok, "Sitekey should be string")
			// Should not contain ENV placeholder (either replaced or empty)
			assert.NotContains(t, sitekeyStr, "$ENV.", "Sitekey should not contain ENV placeholder")
			t.Logf("Captcha sitekey after ENV replacement: %s", sitekeyStr)
		}

		if secret, hasSecret := fullConfig.Form.Captcha.Options["secret"]; hasSecret {
			secretStr, ok := secret.(string)
			assert.True(t, ok, "Secret should be string")
			// Should not contain ENV placeholder (either replaced or empty)
			assert.NotContains(t, secretStr, "$ENV.", "Secret should not contain ENV placeholder")
			t.Logf("Captcha secret after ENV replacement: %s", secretStr)
		}
	}

	// Test OAuth provider ENV variable replacement
	if fullConfig.ThirdParty != nil && fullConfig.ThirdParty.Providers != nil {
		for _, provider := range fullConfig.ThirdParty.Providers {
			// Check ClientID replacement
			if provider.ClientID != "" {
				assert.NotContains(t, provider.ClientID, "$ENV.", "ClientID should not contain ENV placeholder")
				t.Logf("Provider %s ClientID after ENV replacement: %s", provider.ID, provider.ClientID)
			}

			// Check ClientSecret replacement
			if provider.ClientSecret != "" {
				assert.NotContains(t, provider.ClientSecret, "$ENV.", "ClientSecret should not contain ENV placeholder")
				t.Logf("Provider %s ClientSecret after ENV replacement: [REDACTED]", provider.ID)
			}
		}
	}

	// Test that public config doesn't expose ENV variables or actual sensitive values
	publicConfig := signin.GetPublicConfig("")
	assert.NotNil(t, publicConfig, "Should have a public signin configuration")

	// Public config should not contain sensitive data even if ENV variables are set
	if publicConfig.Form != nil && publicConfig.Form.Captcha != nil && publicConfig.Form.Captcha.Options != nil {
		_, hasSecret := publicConfig.Form.Captcha.Options["secret"]
		assert.False(t, hasSecret, "Public config should not contain captcha secret")
	}

	if publicConfig.ThirdParty != nil && publicConfig.ThirdParty.Providers != nil {
		for _, provider := range publicConfig.ThirdParty.Providers {
			assert.Empty(t, provider.ClientID, "Public config should not contain ClientID")
			assert.Empty(t, provider.ClientSecret, "Public config should not contain ClientSecret")
		}
	}
}

func TestSigninENVVariableMissingHandling(t *testing.T) {
	// This test verifies that missing ENV variables are handled securely
	// by returning empty strings instead of exposing the placeholder

	// Initialize test environment
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	_ = serverURL // Server URL not needed for this test

	// Load signin configurations
	err := signin.Load(config.Conf)
	assert.NoError(t, err, "signin.Load should succeed")

	// Get public config (this is what the API returns)
	publicConfig := signin.GetPublicConfig("")
	assert.NotNil(t, publicConfig, "Should have a public signin configuration")

	// Verify that even if ENV variables are missing, no placeholders are exposed
	if publicConfig.Form != nil && publicConfig.Form.Captcha != nil && publicConfig.Form.Captcha.Options != nil {
		for key, value := range publicConfig.Form.Captcha.Options {
			if valueStr, ok := value.(string); ok {
				assert.NotContains(t, valueStr, "$ENV.", "Public config should not contain ENV placeholders in %s", key)
			}
		}
	}

	if publicConfig.ThirdParty != nil && publicConfig.ThirdParty.Providers != nil {
		for _, provider := range publicConfig.ThirdParty.Providers {
			// These should be empty in public config anyway, but verify no ENV placeholders
			assert.NotContains(t, provider.ClientID, "$ENV.", "Public config ClientID should not contain ENV placeholders")
			assert.NotContains(t, provider.ClientSecret, "$ENV.", "Public config ClientSecret should not contain ENV placeholders")
		}
	}

	t.Log("ENV variable security test passed: no ENV placeholders exposed in public configuration")
}
