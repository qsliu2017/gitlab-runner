package common

import io "io"

type Collector interface {
	Collect() io.Reader
}
