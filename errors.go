package main

import "errors"

var ErrNotConnected = errors.New("no active Snowflake connection")
