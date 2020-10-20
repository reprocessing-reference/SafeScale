package openstack

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/gophercloud/gophercloud"

	"github.com/CS-SI/SafeScale/lib/utils/fail"
)

func TestGetUnexpectedGophercloudErrorCode(t *testing.T) {
	type args struct {
		err error
	}

	raw404 := gophercloud.ErrDefault404{}
	raw404.Actual = 404

	refRaw404 := &gophercloud.ErrDefault404{}
	refRaw404.Actual = 404

	othererr := gophercloud.ErrMissingPassword{}
	othererrRef := &gophercloud.ErrMissingPassword{}

	tests := []struct {
		name    string
		args    args
		want    int64
		wantErr bool
	}{
		{"not a gophercloud error, it should fail", args{fmt.Errorf("foo")}, 0, true},
		{"a gophercloud error, it should be 404, nil", args{raw404}, 404, false},
		{"a gophercloud error by reference, it should be 404, nil", args{refRaw404}, 404, false},
		{"a gophercloud native error, it should be 0, err", args{othererr}, 0, true},
		{"a gophercloud native error by ref, it should be 0, err", args{othererrRef}, 0, true},
		{"nil error, it should be 0, err", args{nil}, 0, true},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got, err := GetUnexpectedGophercloudErrorCode(tt.args.err)
				if (err != nil) != tt.wantErr {
					t.Errorf("GetUnexpectedGophercloudErrorCode() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if got != tt.want {
					t.Errorf("GetUnexpectedGophercloudErrorCode() got = %v, want %v", got, tt.want)
				}
			},
		)
	}
}

func Test_gophercloudErrToFail(t *testing.T) {
	type args struct {
		err error
	}

	fooErr := fmt.Errorf("foo")

	raw404 := gophercloud.ErrDefault404{}
	raw404.Actual = 404

	refRaw404 := &gophercloud.ErrDefault404{}
	refRaw404.Actual = 404

	othererr := gophercloud.ErrMissingPassword{}
	othererrRef := &gophercloud.ErrMissingPassword{}

	tests := []struct {
		name    string
		args    args
		want    fail.Error
		wantErr bool
	}{
		{"not a gophercloud error, it should fail", args{fooErr}, fail.NewError("unhandled error received from provider: %s", fooErr.Error()), false},
		{"a gophercloud http error, nil, true", args{raw404}, nil, true},
		{"a gophercloud http error, nil, true", args{refRaw404}, nil, true},
		{"a gophercloud error, it should be 404, false", args{othererr}, fail.NewError("unhandled error received from provider: %s", "You must provide a password to authenticate"), false},
		{"a gophercloud error, it should be 404, false", args{othererrRef}, fail.NewError("unhandled error received from provider: %s", "You must provide a password to authenticate"), false},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got, err := gophercloudErrToFail(tt.args.err)
				if (err != nil) != tt.wantErr {
					t.Errorf("gophercloudErrToFail() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if got != nil && tt.want != nil {
					if !reflect.DeepEqual(got.Error(), tt.want.Error()) {
						t.Errorf("gophercloudErrToFail() got = %v, want %v", got, tt.want)
					}
				}
				if got == nil && tt.want != nil {
					t.Errorf("gophercloudErrToFail() got = %v, want %v", got, tt.want)
				}
				if got != nil && tt.want == nil {
					t.Errorf("gophercloudErrToFail() got = %v, want %v", got, tt.want)
				}
			},
		)
	}
}
