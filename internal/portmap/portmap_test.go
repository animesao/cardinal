package portmap

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    []Mapping
		wantErr bool
	}{
		{
			name: "single mapping tcp default",
			spec: "8080:80",
			want: []Mapping{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
		},
		{
			name: "protocol",
			spec: "5353:53/udp",
			want: []Mapping{{HostPort: 5353, ContainerPort: 53, Protocol: "udp"}},
		},
		{
			name: "host range to single container",
			spec: "8000-8002:80",
			want: []Mapping{
				{HostPort: 8000, ContainerPort: 80, Protocol: "tcp"},
				{HostPort: 8001, ContainerPort: 80, Protocol: "tcp"},
				{HostPort: 8002, ContainerPort: 80, Protocol: "tcp"},
			},
		},
		{
			name: "host and container range 1:1",
			spec: "8000-8002:9000-9002",
			want: []Mapping{
				{HostPort: 8000, ContainerPort: 9000, Protocol: "tcp"},
				{HostPort: 8001, ContainerPort: 9001, Protocol: "tcp"},
				{HostPort: 8002, ContainerPort: 9002, Protocol: "tcp"},
			},
		},
		{
			name: "single host to container range",
			spec: "80:8000-8002",
			want: []Mapping{
				{HostPort: 80, ContainerPort: 8000, Protocol: "tcp"},
				{HostPort: 80, ContainerPort: 8001, Protocol: "tcp"},
				{HostPort: 80, ContainerPort: 8002, Protocol: "tcp"},
			},
		},
		{
			name: "container only",
			spec: "9000",
			want: []Mapping{{HostPort: 0, ContainerPort: 9000, Protocol: "tcp"}},
		},
		{
			name: "container only with protocol",
			spec: "53/udp",
			want: []Mapping{{HostPort: 0, ContainerPort: 53, Protocol: "udp"}},
		},
		{
			name: "host zero random",
			spec: "0:80",
			want: []Mapping{{HostPort: 0, ContainerPort: 80, Protocol: "tcp"}},
		},
		{
			name:    "mismatched range sizes",
			spec:    "8000-8002:9000-9005",
			wantErr: true,
		},
		{
			name:    "reversed range",
			spec:    "8010-8000:80",
			wantErr: true,
		},
		{
			name:    "invalid host port",
			spec:    "http:80",
			wantErr: true,
		},
		{
			name:    "container zero",
			spec:    "80:0",
			wantErr: true,
		},
		{
			name:    "too many fields",
			spec:    "1:2:3",
			wantErr: true,
		},
		{
			name:    "port outside range",
			spec:    "70000:80",
			wantErr: true,
		},
		{
			name: "uppercase protocol normalized",
			spec: "8080:80/UDP",
			want: []Mapping{{HostPort: 8080, ContainerPort: 80, Protocol: "udp"}},
		},
		{
			name:    "garbage protocol rejected (iptables -p injection)",
			spec:    "8080:80/tcp;reboot",
			wantErr: true,
		},
		{
			name: "trailing slash defaults to tcp",
			spec: "8080:80/",
			want: []Mapping{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) expected error, got %v", tt.spec, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.spec, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse(%q) = %v, want %v", tt.spec, got, tt.want)
			}
		})
	}
}
