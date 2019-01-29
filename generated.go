package main 

const (
swagger_content = `{
  "swagger": "2.0",
  "info": {
    "version": "2.1.0",
    "title": "worker-go"
  },
  "schemes": [
    "http"
  ],
  "paths": {
    "/start": {
      "post": {
        "summary": "Start a job in IPED",
        "consumes": [
          "application/json"
        ],
        "produces": [
          "text/plain"
        ],
        "parameters": [
          {
            "in": "body",
            "name": "body",
            "description": "job specification",
            "required": true,
            "schema": {
              "type": "object",
              "properties": {
                "evidence": {
                  "type": "string"
                },
                "output": {
                  "type": "string"
                },
                "profile": {
                  "type": "string"
                }
              }
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success"
          },
          "400": {
            "description": "Bad request"
          }
        }
      }
    }
  }
}`
)
