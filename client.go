package robinhood

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultBaseURL = "https://api.robinhood.com/"
)

type service struct {
	client *Client
}

// Client is an object for making requests to Robinhood API
type Client struct {
	client *http.Client

	BaseURL   *url.URL
	UserName  string
	Password  string
	AuthToken string

	common service

	Accounts    *AccountService
	Auth        *AuthenticationService
	Instruments *InstrumentService
	Positions   *PositionService
	Quotes      *QuoteService
	Trades      *TradeService
}

func NewClient(username, password string) *Client {
	baseURL, _ := url.Parse(defaultBaseURL)
	c := &Client{client: http.DefaultClient, BaseURL: baseURL, UserName: username, Password: password}
	c.common.client = c
	c.Accounts = (*AccountService)(&c.common)
	c.Auth = (*AuthenticationService)(&c.common)
	c.Instruments = (*InstrumentService)(&c.common)
	c.Positions = (*PositionService)(&c.common)
	c.Quotes = (*QuoteService)(&c.common)
	c.Trades = (*TradeService)(&c.common)
	return c
}

func (c *Client) Post(urlStr string, data url.Values, v interface{}) (resp *http.Response, err error) {
	fullUrl, err := c.resolveUrl(urlStr)
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("POST", fullUrl, strings.NewReader(data.Encode()))
	for k, v := range c.getDefaultHeaders() {
		req.Header.Add(k, v)
	}

	if c.AuthToken == "" {
		c.Auth.Login()
	}
	req.Header.Add("Authorization", fmt.Sprintf("Token %v", c.AuthToken))

	return c.Do(req, v)
}

func (c *Client) PostForm(urlStr string, data url.Values, v interface{}) (resp *http.Response, err error) {
	fullUrl, err := c.resolveUrl(urlStr)

	if err != nil {
		return nil, err
	}

	resp, err = c.client.PostForm(fullUrl, data)
	if err != nil {
		return nil, err
	}

	return c.handleResponse(resp, v)
}

func (c *Client) NewRequest(method, urlStr string, body interface{}) (*http.Request, error) {
	fullUrl, err := c.resolveUrl(urlStr)

	if err != nil {
		return nil, err
	}

	return c.NewRequestWithFullUrl(method, fullUrl, body)
}

func (c *Client) NewRequestWithFullUrl(method, fullUrl string, body interface{}) (*http.Request, error) {
	var buf io.ReadWriter
	if body != nil {
		buf = &bytes.Buffer{}
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, fullUrl, buf)
	if err != nil {
		return nil, err
	}

	for k, v := range c.getDefaultHeaders() {
		req.Header.Add(k, v)
	}

	if c.AuthToken == "" {
		c.Auth.Login()
	}
	req.Header.Add("Authorization", fmt.Sprintf("Token %v", c.AuthToken))

	return req, nil
}

func (c *Client) handleResponse(resp *http.Response, v interface{}) (*http.Response, error) {
	defer resp.Body.Close()

	respErr := c.CheckResponse(resp)
	if respErr != nil {
		return resp, respErr
	}

	var err error
	if v != nil {
		err = json.NewDecoder(resp.Body).Decode(v)
		if err == io.EOF {
			err = nil // ignore EOF errors from empty response body
		}
	}

	return resp, err
}

func (c *Client) Do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	return c.handleResponse(resp, v)
}

func (c *Client) resolveUrl(urlStr string) (string, error) {
	rel, err := url.Parse(urlStr)

	if err != nil {
		return "", err
	}

	u := c.BaseURL.ResolveReference(rel)
	return u.String(), err
}

type ErrorResponse struct {
	Response *http.Response
	Body     interface{}
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d %+v",
		r.Response.Request.Method, r.Response.Request.URL,
		r.Response.StatusCode, r.Body)
}

func (c *Client) CheckResponse(r *http.Response) *ErrorResponse {
	s := r.StatusCode

	if 200 <= s && s <= 299 {
		return nil
	}

	if s == http.StatusUnauthorized || s == http.StatusForbidden {
		c.AuthToken = ""
	}

	var f interface{}
	data, err := ioutil.ReadAll(r.Body)
	if err == nil && data != nil {
		json.Unmarshal(data, &f)
	}

	return &ErrorResponse{
		Response: r,
		Body:     f,
	}
}

func (c *Client) getDefaultHeaders() map[string]string {
	defaultHeaders := map[string]string{
		"Accept":                  "*/*",
		"Accept-Language":         "en;q=1, fr;q=0.9, de;q=0.8, ja;q=0.7, nl;q=0.6, it;q=0.5",
		"Content-Type":            "application/x-www-form-urlencoded",
		"X-Robinhood-API-Version": "1.91.1",
		"Connection":              "keep-alive",
		"User-Agent":              "Robinhood/823 (iPhone; iOS 7.1.2; Scale/2.00)",
	}

	return defaultHeaders
}
