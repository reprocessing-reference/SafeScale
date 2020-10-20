/*
 * Copyright 2018-2020, CS Systemes d'Information, http://csgroup.eu
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package openstack

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"

	"github.com/gophercloud/gophercloud"

	"github.com/CS-SI/SafeScale/lib/utils/debug/callstack"
	"github.com/CS-SI/SafeScale/lib/utils/fail"
)

func gophercloudErrToFail(err error) (fail.Error, error) {
	alreadyHandled, newErr := handleNonGophercloudErrors(err)
	if alreadyHandled {
		return newErr, nil
	}

	// FIXME Add other native gophercloud errors
	switch e := err.(type) {
	case gophercloud.ErrResourceNotFound:
		return fail.NotFoundError(e.Error()), nil
	case *gophercloud.ErrResourceNotFound:
		return fail.NotFoundError(e.Error()), nil
	case gophercloud.ErrMultipleResourcesFound:
		return fail.DuplicateError(e.Error()), nil
	case *gophercloud.ErrMultipleResourcesFound:
		return fail.DuplicateError(e.Error()), nil
	default:
		if code, cerr := GetUnexpectedGophercloudErrorCode(err); code != 0 && cerr == nil {
			return nil, fmt.Errorf("this function only handles gophercloud errors WITHOUT http error code")
		}

		logrus.Warnf(callstack.DecorateWith("", "", fmt.Sprintf("Unhandled error (%s) received from provider: %s", reflect.TypeOf(err).String(), err.Error()), 0))
		return fail.NewError("unhandled error received from provider: %s", err.Error()), nil
	}
}

func gophercloudErrWithCodeToFail(code int, err error) fail.Error {
	alreadyHandled, newErr := handleNonGophercloudErrors(err)
	if alreadyHandled {
		return newErr
	}

	switch code {
	case 400:
		return fail.InvalidRequestError(err.Error())
	case 401:
		return fail.NotAuthenticatedError(err.Error())
	case 403:
		return fail.ForbiddenError(err.Error())
	case 404:
		return fail.NotFoundError(err.Error())
	case 408:
		return fail.TimeoutError(err, 0)
	case 409:
		return fail.InvalidRequestError(err.Error())
	case 410:
		return fail.NotFoundError(err.Error())
	case 425:
		return fail.OverloadError(err.Error())
	case 429:
		return fail.OverloadError(err.Error())
	case 500:
		return fail.ExecutionError(nil, err.Error())
	case 503:
		return fail.NotAvailableError(err.Error())
	case 504:
		return fail.NotAvailableError(err.Error())
	default:
		logrus.Warnf(callstack.DecorateWith("", "", fmt.Sprintf("Unhandled error (%s) received from provider: %s", reflect.TypeOf(err).String(), err.Error()), 0))
		return fail.NewError("unhandled error received from provider: %s", err.Error())
	}
}

func NormalizeError(err error) fail.Error {
	return defaultErrorInterpreter(err)
}

// errorMeansServiceUnavailable tells of err contains "service unavailable" (lower/upper/mixed case)
func errorMeansServiceUnavailable(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "service unavailable")
}

// ParseNeutronError parses neutron json error and returns fields
// Returns (nil, fail.ErrSyntax) if json syntax error occured (and maybe operation should be retried...)
// Returns (nil, fail.Error) if any other error occurs
// Returns (<retval>, nil) if everything is understood
func ParseNeutronError(neutronError string) (map[string]string, fail.Error) {
	startIdx := strings.Index(neutronError, "{\"NeutronError\":")
	jsonError := strings.Trim(neutronError[startIdx:], " ")
	unjsoned := map[string]map[string]interface{}{}
	if err := json.Unmarshal([]byte(jsonError), &unjsoned); err != nil {
		switch err.(type) {
		case *json.SyntaxError:
			return nil, fail.SyntaxError(err.Error())
		default:
			logrus.Debugf(err.Error())
			return nil, fail.ToError(err)
		}
	}
	if content, ok := unjsoned["NeutronError"]; ok {
		retval := map[string]string{
			"message": "",
			"type":    "",
			"code":    "",
			"detail":  "",
		}
		if field, ok := content["message"].(string); ok {
			retval["message"] = field
		}
		if field, ok := content["type"].(string); ok {
			retval["type"] = field
		}
		if field, ok := content["code"].(string); ok {
			retval["code"] = field
		}
		if field, ok := content["detail"].(string); ok {
			retval["detail"] = field
		}

		return retval, nil
	}
	return nil, nil
}

func caseInsensitiveContains(haystack, needle string) bool {
	lowerHaystack := strings.ToLower(haystack)
	lowerNeedle := strings.ToLower(needle)

	return strings.Contains(lowerHaystack, lowerNeedle)
}

func IsServiceUnavailableError(err error) bool {
	if err != nil {
		text := err.Error()
		return caseInsensitiveContains(text, "Service Unavailable")
	}

	return false
}

func GetUnexpectedGophercloudErrorCode(err error) (int64, error) {
	xType := reflect.TypeOf(err)
	xValue := reflect.ValueOf(err)

	if xValue.Kind() == reflect.Ptr && !xValue.IsNil() {
		xValue = xValue.Elem()
		xType = xValue.Type()
	}

	if xValue.Kind() != reflect.Struct {
		return 0, fail.Errorf(nil, fmt.Sprintf("not a gophercloud.ErrUnexpectedResponseCode"))
	}

	_, there := xType.FieldByName("ErrUnexpectedResponseCode")
	if there {
		_, there := xType.FieldByName("Actual")
		if there {
			recoveredValue := xValue.FieldByName("Actual").Int()
			if recoveredValue != 0 {
				return recoveredValue, nil
			}
		}
	}

	return 0, fail.Errorf(nil, fmt.Sprintf("not a gophercloud.ErrUnexpectedResponseCode"))
}

func handleNonGophercloudErrors(gopherErr error) (handled bool, err fail.Error) {
	if gopherErr == nil {
		return true, nil
	}

	if casted, ok := gopherErr.(fail.Error); ok {
		return true, casted
	}

	if casted, ok := gopherErr.(*url.Error); ok {
		if casted.Timeout() {
			return true, fail.TimeoutError(gopherErr, 0)
		}

		if casted.Temporary() {
			return true, fail.OverloadError(gopherErr.Error())
		}

		return true, fail.NewErrorWithCause(gopherErr)
	}

	return false, nil
}

func ReinterpretGophercloudErrorCode(gopherErr error, success []int64, transparent []int64, abort []int64, defaultHandler func(error) fail.Error) fail.Error {
	alreadyHandled, newErr := handleNonGophercloudErrors(gopherErr)
	if alreadyHandled {
		return newErr
	}

	if code, err := GetUnexpectedGophercloudErrorCode(gopherErr); code != 0 && err == nil {
		for _, tcode := range success {
			if tcode == code {
				return nil
			}
		}

		for _, tcode := range abort {
			if tcode == code {
				logrus.Warnf("received code %d, we have to abort", code)
				return fail.AbortedError(gophercloudErrWithCodeToFail(int(code), gopherErr), "")
			}
		}

		for _, tcode := range transparent {
			if tcode == code {
				logrus.Warnf("received code %d, the error goes trough", code)
				return gophercloudErrWithCodeToFail(int(code), gopherErr)
			}
		}

		if defaultHandler == nil {
			return nil
		}

		return defaultHandler(gopherErr)
	}

	mapped, mappingProblem := gophercloudErrToFail(gopherErr)
	if mappingProblem != nil {
		logrus.Warnf(mappingProblem.Error())
		return fail.Wrap(gopherErr)
	}

	return mapped
}

func defaultErrorInterpreter(inErr error) fail.Error {
	return ReinterpretGophercloudErrorCode(
		inErr, nil, []int64{408, 409, 425, 429, 500, 503, 504}, nil, func(ferr error) fail.Error {
			if IsServiceUnavailableError(ferr) {
				return fail.NotAvailableError(ferr.Error())
			}

			if ferr, ok := ferr.(fail.Error); ok {
				logrus.Warn("forwarding error")
				return ferr
			}

			if code, cerr := GetUnexpectedGophercloudErrorCode(ferr); code != 0 && cerr == nil {
				logrus.Warn("fallback to transparent errors")
				return gophercloudErrWithCodeToFail(int(code), ferr)
			}

			logrus.Warnf("wrapping error: %s", spew.Sdump(ferr))
			return fail.Wrap(ferr)
		},
	)
}
