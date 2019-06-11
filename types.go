package main

import "time"

type Token struct {
	Path       string     `json:"path"`
	Token      string     `json:"token"`
	Expiration *time.Time `json:"expiration"`
}

type Frozen struct {
	Path       string     `json:"path"`
	Expiration *time.Time `json:"expiration"`
}

type Domain struct {
	Fqdn       string              `json:"fqdn"`
	Hosts      []string            `json:"hosts,omitempty"`
	SubDomain  map[string][]string `json:"subdomain,omitempty"`
	Text       string              `json:"text,omitempty"`
	Token      string              `json:"token,omitempty"`
	Expiration *time.Time          `json:"expiration"`
}

type Response struct {
	Status  int    `json:"status"`
	Message string `json:"msg"`
	Data    Domain `json:"data,omitempty"`
	Token   string `json:"token"`
}
