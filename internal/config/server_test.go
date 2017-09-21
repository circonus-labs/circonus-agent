// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"errors"
	"testing"
)

func TestParseListen(t *testing.T) {
	t.Log("Testing parseListen")

	t.Log("No spec, no default spec")
	{
		ip, port, err := parseListen("", "")
		if err != nil {
			t.Errorf("Expected no error got %v", err)
		}
		if ip != "" {
			t.Errorf("Expected blank ip, got '%s'", ip)
		}
		if port != "" {
			t.Errorf("Expected blank port, got '%s'", port)
		}
	}

	t.Log("Invalid default spec IP")
	{
		expectedErr := errors.New("Invalid IP address format specified 'foo'")
		ip, port, err := parseListen("", "foo:")
		if err == nil {
			t.Errorf("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
		if ip != "" {
			t.Errorf("Expected blank ip, got '%s'", ip)
		}
		if port != "" {
			t.Errorf("Expected blank port, got '%s'", port)
		}
	}

	t.Log("Invalid default spec Port")
	{
		expectedErr := errors.New("Invalid port: strconv.Atoi: parsing \"foo\": invalid syntax")
		ip, port, err := parseListen("", ":foo")
		if err == nil {
			t.Errorf("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
		if ip != "" {
			t.Errorf("Expected blank ip, got '%s'", ip)
		}
		if port != "" {
			t.Errorf("Expected blank port, got '%s'", port)
		}
	}

	t.Log("Invalid default spec ':'")
	{
		expectedErr := errors.New("Missing IP () and Port () in specification ()")
		ip, port, err := parseListen("", ":")
		if err == nil {
			t.Errorf("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
		if ip != "" {
			t.Errorf("Expected blank ip, got '%s'", ip)
		}
		if port != "" {
			t.Errorf("Expected blank port, got '%s'", port)
		}
	}

	t.Log("Invalid spec IP")
	{
		expectedErr := errors.New("Invalid IP address format specified 'foo'")
		ip, port, err := parseListen("foo:", "")
		if err == nil {
			t.Errorf("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
		if ip != "" {
			t.Errorf("Expected blank ip, got '%s'", ip)
		}
		if port != "" {
			t.Errorf("Expected blank port, got '%s'", port)
		}
	}

	t.Log("Invalid spec Port")
	{
		expectedErr := errors.New("Invalid port: strconv.Atoi: parsing \"foo\": invalid syntax")
		ip, port, err := parseListen(":foo", "")
		if err == nil {
			t.Errorf("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
		if ip != "" {
			t.Errorf("Expected blank ip, got '%s'", ip)
		}
		if port != "" {
			t.Errorf("Expected blank port, got '%s'", port)
		}
	}

	t.Log("Invalid spec ':'")
	{
		expectedErr := errors.New("Missing IP () and Port () in specification (:)")
		ip, port, err := parseListen(":", "")
		if err == nil {
			t.Errorf("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
		if ip != "" {
			t.Errorf("Expected blank ip, got '%s'", ip)
		}
		if port != "" {
			t.Errorf("Expected blank port, got '%s'", port)
		}
	}

	t.Log("Invalid port (low)")
	{
		expectedErr := errors.New("Invalid port, out of range 0<-1<65535")
		ip, port, err := parseListen("", ":-1")
		if err == nil {
			t.Errorf("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
		if ip != "" {
			t.Errorf("Expected blank ip, got '%s'", ip)
		}
		if port != "" {
			t.Errorf("Expected blank port, got '%s'", port)
		}
	}

	t.Log("Invalid port (high)")
	{
		expectedErr := errors.New("Invalid port, out of range 0<70000<65535")
		ip, port, err := parseListen("", ":70000")
		if err == nil {
			t.Errorf("Expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected (%s) got (%s)", expectedErr, err)
		}
		if ip != "" {
			t.Errorf("Expected blank ip, got '%s'", ip)
		}
		if port != "" {
			t.Errorf("Expected blank port, got '%s'", port)
		}
	}

	t.Log("Defaults")
	{
		ip, port, err := parseListen("", "2609")
		if err != nil {
			t.Errorf("Expected NO error got (%s)", err)
		}
		if ip != "" {
			t.Errorf("Expected blank ip, got '%s'", ip)
		}
		if port != "2609" {
			t.Errorf("Expected 2609 port, got '%s'", port)
		}
	}

	t.Log("Custom port, override")
	{
		ip, port, err := parseListen("1234", "127.0.0.1:2609")
		if err != nil {
			t.Errorf("Expected NO error got (%s)", err)
		}
		if ip != "127.0.0.1" {
			t.Errorf("Expected 127.0.0.1 ip, got '%s'", ip)
		}
		if port != "1234" {
			t.Errorf("Expected 1234 port, got '%s'", port)
		}
	}

	t.Log("Custom port (none in default spec)")
	{
		ip, port, err := parseListen("1234", "127.0.0.1")
		if err != nil {
			t.Errorf("Expected NO error got (%s)", err)
		}
		if ip != "127.0.0.1" {
			t.Errorf("Expected 127.0.0.1 ip, got '%s'", ip)
		}
		if port != "1234" {
			t.Errorf("Expected 1234 port, got '%s'", port)
		}
	}

	t.Log("Custom IP")
	{
		ip, port, err := parseListen("127.0.0.1", ":2609")
		if err != nil {
			t.Errorf("Expected NO error got (%s)", err)
		}
		if ip != "127.0.0.1" {
			t.Errorf("Expected 127.0.0.1 ip, got '%s'", ip)
		}
		if port != "2609" {
			t.Errorf("Expected 2609 port, got '%s'", port)
		}
	}

	t.Log("Custom IP & port")
	{
		ip, port, err := parseListen("127.0.0.1:1234", ":2609")
		if err != nil {
			t.Errorf("Expected NO error got (%s)", err)
		}
		if ip != "127.0.0.1" {
			t.Errorf("Expected 127.0.0.1 ip, got '%s'", ip)
		}
		if port != "1234" {
			t.Errorf("Expected 1234 port, got '%s'", port)
		}
	}
}
