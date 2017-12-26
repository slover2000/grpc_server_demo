package thirdparty

import (
	"log"
	"strings"
	"golang.org/x/net/context"
)

const (
	colonSep			= ":"

	SystemFieldIndex 	= 0
	DBFieldIndex		= 1
	TableFieldIndex		= 2
	ActionFieldIndex	= 3
	SQLFieldIndex 		= 4
	
	DBMinTotalParams 	= 4
	DBMaxTotalParams 	= 5

	CacheActionFieldIndex  		 = 1
	CacheCommandOptionFieldIndex = 2
	CacheMinTotalParams = 2
	
	SearchIndexFieldIndex 	 = 1
	SearchDocumentFieldIndex = 2
	SearchActionFieldIndex   = 3
	SearchCommandOptionFieldIndex  = 4
	SearchMinTotalParams = 4
)

type interceptorKey struct{}

// DatabaseParam define database relative params
type DatabaseParam struct {
	System      string
	Database    string
	Table       string
	Action      string
	SQL         string
}

// CacheParam define cache relative params
type CacheParam struct {
	System      string
	Action      string
	Command     string
}

// SearchParam define search relative params
type SearchParam struct {
	System      string
	Action      string
	Index       string
	Document    string
	Command     string
}

// JoinContextValue returns a derived context containing the values.
func JoinContextValue(ctx context.Context, values ...string) context.Context {
	return context.WithValue(ctx, interceptorKey{}, strings.Join(values, colonSep))
}

// parseContextValue parse string
func parseContextValue(ctx context.Context) ([]string, bool) {
	if s, ok := ctx.Value(interceptorKey{}).(string); ok {
		return strings.Split(s, colonSep), true
	}

	return nil, false
}

// ParseDatabaeContextValue parse thirdparty database params from context
func ParseDatabaeContextValue(ctx context.Context) (*DatabaseParam, bool) {
	if values, ok := parseContextValue(ctx); ok {
		if len(values) < DBMinTotalParams {
			log.Printf("wrong number of database params")
			return nil, false
		}

		params := &DatabaseParam{
			System: values[SystemFieldIndex],
			Database: values[DBFieldIndex],
			Table: values[TableFieldIndex],
			Action: values[ActionFieldIndex],
		}

		if len(values) >= DBMaxTotalParams {
			params.SQL = values[SQLFieldIndex]
		}

		return params, true
	}

	log.Printf("failed to parse database params from context")
	return nil, false
}

// ParseCacheContextValue parse thirdparty cache params from context
func ParseCacheContextValue(ctx context.Context) (*CacheParam, bool) {
	if values, ok := parseContextValue(ctx); ok {
		if len(values) < CacheMinTotalParams {
			log.Printf("wrong number of cache params")
			return nil, false
		}

		params := &CacheParam{
			System: values[SystemFieldIndex],
			Action: values[CacheActionFieldIndex],
		}

		if len(values) > CacheMinTotalParams {
			params.Command = values[CacheCommandOptionFieldIndex]
		}

		return params, true
	}

	log.Printf("failed to parse cache params from context")
	return nil, false
}

// ParseSearchContextValue parse thirdparty search params from context
func ParseSearchContextValue(ctx context.Context) (*SearchParam, bool) {
	if values, ok := parseContextValue(ctx); ok {
		if len(values) < SearchMinTotalParams {
			log.Printf("wrong number of search params")
			return nil, false
		}

		params := &SearchParam{
			System: values[SystemFieldIndex],
			Index: values[SearchIndexFieldIndex],
			Document: values[SearchDocumentFieldIndex],
			Action: values[SearchActionFieldIndex],
		}

		if len(values) > SearchMinTotalParams {
			params.Command = values[SearchCommandOptionFieldIndex]
		}

		return params, true
	}

	log.Printf("failed to parse search params from context")
	return nil, false
}