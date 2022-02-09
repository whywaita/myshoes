# myshoes-sdk-go

The Go SDK for myshoes

## Usage

```go
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	
	"github.com/whywaita/myshoes/api/myshoes"
)

func main()  {
	// Set customized HTTP Client
	customHTTPClient := http.DefaultClient
	// Set customized logger
	customLogger := log.New(io.Discard, "", log.LstdFlags)
	
    client, err := myshoes.NewClient("https://example.com", customHTTPClient, customLogger)
	if err != nil {
		// ...
    }
	
	targets, err := client.ListTarget(context.Background())
	if err != nil {
		// ...
    }
	
	fmt.Println(targets)
}
```