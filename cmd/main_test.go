package kenginecmd

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	for i, tc := range []struct {
		input     string
		expect    map[string]string
		shouldErr bool
	}{
		{
			input: `KEY=value`,
			expect: map[string]string{
				"KEY": "value",
			},
		},
		{
			input: `
				KEY=value
				OTHER_KEY=Some Value
			`,
			expect: map[string]string{
				"KEY":       "value",
				"OTHER_KEY": "Some Value",
			},
		},
		{
			input: `
				KEY=value
				INVALID KEY=asdf
				OTHER_KEY=Some Value
			`,
			shouldErr: true,
		},
		{
			input: `
				KEY=value
				SIMPLE_QUOTED="quoted value"
				OTHER_KEY=Some Value
			`,
			expect: map[string]string{
				"KEY":           "value",
				"SIMPLE_QUOTED": "quoted value",
				"OTHER_KEY":     "Some Value",
			},
		},
		{
			input: `
				KEY=value
				NEWLINES="foo
	bar"
				OTHER_KEY=Some Value
			`,
			expect: map[string]string{
				"KEY":       "value",
				"NEWLINES":  "foo\n\tbar",
				"OTHER_KEY": "Some Value",
			},
		},
		{
			input: `
				KEY=value
				ESCAPED="\"escaped quotes\"
here"
				OTHER_KEY=Some Value
			`,
			expect: map[string]string{
				"KEY":       "value",
				"ESCAPED":   "\"escaped quotes\"\nhere",
				"OTHER_KEY": "Some Value",
			},
		},
		{
			input: `
				export KEY=value
				OTHER_KEY=Some Value
			`,
			expect: map[string]string{
				"KEY":       "value",
				"OTHER_KEY": "Some Value",
			},
		},
		{
			input: `
				=value
				OTHER_KEY=Some Value
			`,
			shouldErr: true,
		},
		{
			input: `
				EMPTY=
				OTHER_KEY=Some Value
			`,
			expect: map[string]string{
				"EMPTY":     "",
				"OTHER_KEY": "Some Value",
			},
		},
		{
			input: `
				EMPTY=""
				OTHER_KEY=Some Value
			`,
			expect: map[string]string{
				"EMPTY":     "",
				"OTHER_KEY": "Some Value",
			},
		},
		{
			input: `
				KEY=value
				#OTHER_KEY=Some Value
			`,
			expect: map[string]string{
				"KEY": "value",
			},
		},
		{
			input: `
				KEY=value
				COMMENT=foo bar  # some comment here
				OTHER_KEY=Some Value
			`,
			expect: map[string]string{
				"KEY":       "value",
				"COMMENT":   "foo bar",
				"OTHER_KEY": "Some Value",
			},
		},
		{
			input: `
				KEY=value
				WHITESPACE=   foo 
				OTHER_KEY=Some Value
			`,
			shouldErr: true,
		},
		{
			input: `
				KEY=value
				WHITESPACE="   foo bar "
				OTHER_KEY=Some Value
			`,
			expect: map[string]string{
				"KEY":        "value",
				"WHITESPACE": "   foo bar ",
				"OTHER_KEY":  "Some Value",
			},
		},
	} {
		actual, err := parseEnvFile(strings.NewReader(tc.input))
		if err != nil && !tc.shouldErr {
			t.Errorf("Test %d: Got error but shouldn't have: %v", i, err)
		}
		if err == nil && tc.shouldErr {
			t.Errorf("Test %d: Did not get error but should have", i)
		}
		if tc.shouldErr {
			continue
		}
		if !reflect.DeepEqual(tc.expect, actual) {
			t.Errorf("Test %d: Expected %v but got %v", i, tc.expect, actual)
		}
	}
}

func Test_isKenginefile(t *testing.T) {
	type args struct {
		configFile  string
		adapterName string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "bare Kenginefile without adapter",
			args: args{
				configFile:  "Kenginefile",
				adapterName: "",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "local Kenginefile without adapter",
			args: args{
				configFile:  "./Kenginefile",
				adapterName: "",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "local kenginefile with adapter",
			args: args{
				configFile:  "./Kenginefile",
				adapterName: "kenginefile",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "ends with .kenginefile with adapter",
			args: args{
				configFile:  "./conf.kenginefile",
				adapterName: "kenginefile",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "ends with .kenginefile without adapter",
			args: args{
				configFile:  "./conf.kenginefile",
				adapterName: "",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "config is Kenginefile.yaml with adapter",
			args: args{
				configFile:  "./Kenginefile.yaml",
				adapterName: "yaml",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "json is not kenginefile but not error",
			args: args{
				configFile:  "./Kenginefile.json",
				adapterName: "",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "prefix of Kenginefile and ./ with any extension is Kenginefile",
			args: args{
				configFile:  "./Kenginefile.prd",
				adapterName: "",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "prefix of Kenginefile without ./ with any extension is Kenginefile",
			args: args{
				configFile:  "Kenginefile.prd",
				adapterName: "",
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isKenginefile(tt.args.configFile, tt.args.adapterName)
			if (err != nil) != tt.wantErr {
				t.Errorf("isKenginefile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isKenginefile() = %v, want %v", got, tt.want)
			}
		})
	}
}
