package thirdparty

import (
	"log"
	"fmt"
	"strings"
	"testing"
	"golang.org/x/net/context"
)

func TestContext(t *testing.T) {
	ctx := JoinContextValue(context.Background(), "mongo", "mishop", "orders", "query")
	values, _ := parseContextValue(ctx)
	log.Println(fmt.Sprintf("value:%s", strings.Join(values, ":")))

	values, _ = parseContextValue(context.Background())
	log.Println(fmt.Sprintf("value:%s", strings.Join(values, ":")))
}