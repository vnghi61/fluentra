package cache

import "fmt"

// Key builds a standardized cache key adhering to repo conventions:
// fluentra:{env}:{module}:{entity}:{id}:v{version}
func Key(env, module, entity, id string, version int) string {
	return fmt.Sprintf("fluentra:%s:%s:%s:%s:v%d", env, module, entity, id, version)
}
