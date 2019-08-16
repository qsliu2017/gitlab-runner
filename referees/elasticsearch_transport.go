// The only purpose for having this files and interfaces redefined
// in it is to make automatic mocks generator (`make mocks`) able to
// create mocks of some Elasticsearch interfaces - which are not present
// in the original packages but are required to make our tests simpler
// and more "unit".

package referees

import (
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

type transport interface {
	esapi.Transport
}
