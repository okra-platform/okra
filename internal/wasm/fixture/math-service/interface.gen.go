// Code generated from GraphQL schema. DO NOT EDIT.

package main

// AddInput represents the input for the add method
type AddInput struct {
	A int `json:"a"`
	B int `json:"b"`
}

// AddResponse represents the response from the add method
type AddResponse struct {
	Sum int `json:"sum"`
}

// MathService defines the service interface
type MathService interface {
	Add(input *AddInput) (*AddResponse, error)
}