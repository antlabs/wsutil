package hostname

import (
	"net/url"
	"testing"
)

type test_getHostName struct {
	data string
	need string
}

func Test_getHostName(t *testing.T) {
	t.Run("test 1", func(t *testing.T) {
		for _, d := range []test_getHostName{
			{
				data: "http://www.baidu.com",
				need: "www.baidu.com:80",
			},
			{
				data: "http://www.baidu.com:333",
				need: "www.baidu.com:333",
			},
			{
				data: "https://www.baidu.com",
				need: "www.baidu.com:443",
			},
		} {

			u, err := url.Parse(d.data)
			if err != nil {
				t.Errorf("err should be nil, got %s", err)
			}
			if GetHostName(u) != d.need {
				t.Errorf("need %s, got %s", d.need, GetHostName(u))
			}
		}
	})
}
