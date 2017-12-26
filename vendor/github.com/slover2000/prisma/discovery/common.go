package discovery

// GRPCSystem is leading path of gprc service
const HTTPSystem = "http"
const GRPCSystem = "gprc"
const RedisSystem = "redis"
const MysqlSystem = "mysql"
const MongoSystem = "mongo"

// DefaultGroup is default group name
const DefaultGroup = "DEFAULT_GROUP"

// DefaultWeight is default weight of endpoint
const DefaultWeight = 5

// DefaultOpTimeout is default timeout for each etcd operation in seconds
const DefaultRequestTimeout = 5

type EnvironmentType = int
// iota 初始化后会自动递增
const (
	Staging EnvironmentType = iota
	Preview
	Product
)

type (
	Endpoint struct {
		Host	string	`json:"host"`
		Port	int		`json:"port"`
		Weight	int		`json:"weight"`
		EnvType int		`json:"envtype"`
		Group 	string	`json:"group"`
	}
)

func EnviormentTypeToString(t EnvironmentType) string {
	switch t {
	case Staging:
		return "staging"
	case Preview:
		return "preview"
	case Product:
		return "product"
	default:
		return "unknown"
	}
}