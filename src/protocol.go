package main

type Protocol interface {
	Run()
	Stop()
}
