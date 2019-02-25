package main

import "errors"

var ErrWSAlreadyOpen = errors.New("web socket already opened")

var ErrWSNotFound = errors.New("no websocket connection exists")
