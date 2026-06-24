package processor

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/free5gc/smf/internal/context"
)

func TestBuildULCLUpLinkFlowDescription(t *testing.T) {
	testCases := []struct {
		name string
		dest context.Destination
		want string
	}{
		{
			name: "CIDR destination",
			dest: context.Destination{
				DestinationIP: "192.168.0.0/16",
			},
			want: "permit out ip from 192.168.0.0/16 to 10.60.0.1",
		},
		{
			name: "CIDR destination with port",
			dest: context.Destination{
				DestinationIP:   "192.168.0.0/16",
				DestinationPort: "8000",
			},
			want: "permit out ip from 192.168.0.0/16 8000 to 10.60.0.1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildULCLUpLinkFlowDescription(net.ParseIP("10.60.0.1"), tc.dest)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
