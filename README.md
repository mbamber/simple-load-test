# Simple Load Test

Simple Load Test is a simple golang application to perform a large number of concurrent requests to a given endpoint

## Why not JMeter etc?

These tools are unecessarily complex for simple use. If all you want to do is send a large number of requests to an endpoint without any of the overhead or complex configuration, then Simple Load Test is for you!

## Usage

It's as simple as: `go run main.go --req-per-second 1000 https://mysite.com/test`

For all options, run `go run main.go --help`
