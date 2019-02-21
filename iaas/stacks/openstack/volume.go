/*
 * Copyright 2018, CS Systemes d'Information, http://www.c-s.fr
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
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	gc "github.com/gophercloud/gophercloud"
	volumesv1 "github.com/gophercloud/gophercloud/openstack/blockstorage/v1/volumes"
	volumesv2 "github.com/gophercloud/gophercloud/openstack/blockstorage/v2/volumes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/volumeattach"
	"github.com/gophercloud/gophercloud/pagination"

	"github.com/CS-SI/SafeScale/iaas/resources"
	"github.com/CS-SI/SafeScale/iaas/resources/enums/VolumeSpeed"
	"github.com/CS-SI/SafeScale/iaas/resources/enums/VolumeState"
	"github.com/CS-SI/SafeScale/utils/retry"
)

// toVolumeState converts a Volume status returned by the OpenStack driver into VolumeState enum
func toVolumeState(status string) VolumeState.Enum {
	switch status {
	case "creating":
		return VolumeState.CREATING
	case "available":
		return VolumeState.AVAILABLE
	case "attaching":
		return VolumeState.ATTACHING
	case "detaching":
		return VolumeState.DETACHING
	case "in-use":
		return VolumeState.USED
	case "deleting":
		return VolumeState.DELETING
	case "error", "error_deleting", "error_backing-up", "error_restoring", "error_extending":
		return VolumeState.ERROR
	default:
		return VolumeState.OTHER
	}
}

func (s *Stack) getVolumeType(speed VolumeSpeed.Enum) string {
	for t, s := range s.cfgOpts.VolumeSpeeds {
		if s == speed {
			return t
		}
	}
	switch speed {
	case VolumeSpeed.SSD:
		return s.getVolumeType(VolumeSpeed.HDD)
	case VolumeSpeed.HDD:
		return s.getVolumeType(VolumeSpeed.COLD)
	default:
		return ""
	}
}

func (s *Stack) getVolumeSpeed(vType string) VolumeSpeed.Enum {
	speed, ok := s.cfgOpts.VolumeSpeeds[vType]
	if ok {
		return speed
	}
	return VolumeSpeed.HDD
}

// CreateVolume creates a block volume
// - name is the name of the volume
// - size is the size of the volume in GB
// - volumeType is the type of volume to create, if volumeType is empty the driver use a default type
func (s *Stack) CreateVolume(request resources.VolumeRequest) (*resources.Volume, error) {
	log.Debugf(">>> stacks.openstack::CreateVolume(%s)", request.Name)
	defer log.Debugf("<<< stacks.openstack::CreateVolume(%s)", request.Name)

	if s == nil {
		panic("Calling openstack.Stack::CreateVolume() from nil pointer!")
	}

	volume, err := s.GetVolume(request.Name)
	if err != nil {
		if _, ok := err.(resources.ErrResourceNotFound); !ok {
			return nil, err
		}
	}
	if volume != nil {
		return nil, fmt.Errorf("volume '%s' already exists", request.Name)
	}

	var v resources.Volume
	switch s.versions["volume"] {
	case "v1":
		var vol *volumesv1.Volume
		vol, err = volumesv1.Create(s.VolumeClient, volumesv1.CreateOpts{
			Name:       request.Name,
			Size:       request.Size,
			VolumeType: s.getVolumeType(request.Speed),
		}).Extract()
		v = resources.Volume{
			ID:    vol.ID,
			Name:  vol.Name,
			Size:  vol.Size,
			Speed: s.getVolumeSpeed(vol.VolumeType),
			State: toVolumeState(vol.Status),
		}
	case "v2":
		var vol *volumesv2.Volume
		vol, err = volumesv2.Create(s.VolumeClient, volumesv2.CreateOpts{
			Name:       request.Name,
			Size:       request.Size,
			VolumeType: s.getVolumeType(request.Speed),
		}).Extract()
		v = resources.Volume{
			ID:    vol.ID,
			Name:  vol.Name,
			Size:  vol.Size,
			Speed: s.getVolumeSpeed(vol.VolumeType),
			State: toVolumeState(vol.Status),
		}
	default:
		return nil, fmt.Errorf("unmanaged service 'volume' version '%s'", s.versions["volume"])
	}
	if err != nil {
		log.Debugf("Error creating volume: volume creation invocation: %+v", err)
		return nil, errors.Wrap(err, fmt.Sprintf("Error creating volume : %s", ProviderErrorToString(err)))
	}

	return &v, nil
}

// GetVolume returns the volume identified by id
func (s *Stack) GetVolume(id string) (*resources.Volume, error) {
	log.Debugf(">>> stacks.openstack::GetVolume(%s)", id)
	defer log.Debugf("<<< stacks.openstack::GetVolume(%s)", id)

	if s == nil {
		panic("Calling stacks.openstack::GetVolume() from nil pointer!")
	}

	r := volumesv2.Get(s.VolumeClient, id)
	volume, err := r.Extract()
	if err != nil {
		if _, ok := err.(gc.ErrDefault404); ok {
			return nil, resources.ResourceNotFoundError("volume", id)
		}
		log.Debugf("Error getting volume: volume query failed: %+v", err)
		return nil, errors.Wrap(err, fmt.Sprintf("Error getting volume: %s", ProviderErrorToString(err)))
	}

	av := resources.Volume{
		ID:    volume.ID,
		Name:  volume.Name,
		Size:  volume.Size,
		Speed: s.getVolumeSpeed(volume.VolumeType),
		State: toVolumeState(volume.Status),
	}
	return &av, nil
}

// ListVolumes returns the list of all volumes known on the current tenant
func (s *Stack) ListVolumes() ([]resources.Volume, error) {
	log.Debug(">>> stacks.openstack::ListVolumes()")
	defer log.Debug("<<< stacks.openstack::ListVolumes()")

	if s == nil {
		panic("Calling stacks.openstack::ListVolumes() from nil pointer!")
	}

	var vs []resources.Volume
	err := volumesv2.List(s.VolumeClient, volumesv2.ListOpts{}).EachPage(func(page pagination.Page) (bool, error) {
		list, err := volumesv2.ExtractVolumes(page)
		if err != nil {
			log.Errorf("Error listing volumes: volume extraction: %+v", err)
			return false, err
		}
		for _, vol := range list {
			av := resources.Volume{
				ID:    vol.ID,
				Name:  vol.Name,
				Size:  vol.Size,
				Speed: s.getVolumeSpeed(vol.VolumeType),
				State: toVolumeState(vol.Status),
			}
			vs = append(vs, av)
		}
		return true, nil
	})
	if err != nil || len(vs) == 0 {
		if err != nil {
			log.Debugf("Error listing volumes: list invocation: %+v", err)
			return nil, errors.Wrap(err, fmt.Sprintf("Error listing volume types: %s", ProviderErrorToString(err)))
		}
		log.Warnf("Complete volume list empty")
	}
	return vs, nil
}

// DeleteVolume deletes the volume identified by id
func (s *Stack) DeleteVolume(id string) error {
	log.Debugf(">>> stacks.openstack::DeleteVolume(%s)", id)
	defer log.Debugf("<<< stacks.openstack::DeleteVolume(%s)", id)

	if s == nil {
		panic("Calling openstack.Stack::DeleteVolume() from nil pointer!")
	}

	var (
		err     error
		timeout = 30 * time.Second
	)

	retryErr := retry.WhileUnsuccessfulDelay5Seconds(
		func() error {
			r := volumesv2.Delete(s.VolumeClient, id, nil)
			err := r.ExtractErr()
			if err != nil {
				switch err.(type) {
				case gc.ErrDefault400:
					return fmt.Errorf("volume not in state 'available'")
				default:
					return err
				}
			}
			return nil
		},
		timeout,
	)
	if retryErr != nil {
		switch retryErr.(type) {
		case retry.ErrTimeout:
			if err != nil {
				return fmt.Errorf("timeout after %v to delete volume: %v", timeout, err)
			}
		}
		log.Debugf("Error deleting volume: %+v", retryErr)
		return errors.Wrap(retryErr, fmt.Sprintf("Error deleting volume: %v", retryErr))
	}
	return nil
}

// CreateVolumeAttachment attaches a volume to an host
// - 'name' of the volume attachment
// - 'volume' to attach
// - 'host' on which the volume is attached
func (s *Stack) CreateVolumeAttachment(request resources.VolumeAttachmentRequest) (string, error) {
	log.Debugf(">>> stacks.openstack::CreateVolumeAttachment(%s)", request.Name)
	defer log.Debugf("<<< stacks.openstack::CreateVolumeAttachment(%s)", request.Name)

	if s == nil {
		panic("Calling stacks.openstack::CreateVolumeAttachment() from nil pointer!")
	}

	// Creates the attachment
	r := volumeattach.Create(s.ComputeClient, request.HostID, volumeattach.CreateOpts{
		VolumeID: request.VolumeID,
	})
	va, err := r.Extract()
	if err != nil {
		spew.Dump(r.Err)
		// switch r.Err.(type) {
		// 	case
		// }
		// message := extractMessageFromBadRequest(r.Err)
		// if message != ""
		log.Debugf("Error creating volume attachment: actual attachment creation: %+v", err)
		return "", errors.Wrap(err, fmt.Sprintf("Error creating volume attachment between server %s and volume %s: %s", request.HostID, request.VolumeID, ProviderErrorToString(err)))
	}

	return va.ID, nil
}

// GetVolumeAttachment returns the volume attachment identified by id
func (s *Stack) GetVolumeAttachment(serverID, id string) (*resources.VolumeAttachment, error) {
	log.Debugf(">>> stacks.openstack::GetVolumeAttachment(%s)", id)
	defer log.Debugf("<<< stacks.openstack::GetVolumeAttachment(%s)", id)

	if s == nil {
		panic("Calling stacks.openstack::GetVolumeAttachment() from nil pointer!")
	}

	va, err := volumeattach.Get(s.ComputeClient, serverID, id).Extract()
	if err != nil {
		log.Debugf("Error getting volume attachment: get call: %+v", err)
		return nil, errors.Wrap(err, fmt.Sprintf("Error getting volume attachment %s: %s", id, ProviderErrorToString(err)))
	}
	return &resources.VolumeAttachment{
		ID:       va.ID,
		ServerID: va.ServerID,
		VolumeID: va.VolumeID,
		Device:   va.Device,
	}, nil
}

// ListVolumeAttachments lists available volume attachment
func (s *Stack) ListVolumeAttachments(serverID string) ([]resources.VolumeAttachment, error) {
	log.Debugf(">>> stacks.openstack::ListVolumeAttachments(%s)", serverID)
	defer log.Debugf("<<< stacks.openstack::ListVolumeAttachments(%s)", serverID)

	if s == nil {
		panic("Calling stacks.openstack::ListVolumeAttachments() from nil pointer!")
	}

	var vs []resources.VolumeAttachment
	err := volumeattach.List(s.ComputeClient, serverID).EachPage(func(page pagination.Page) (bool, error) {
		list, err := volumeattach.ExtractVolumeAttachments(page)
		if err != nil {
			log.Debugf("Error listing volume attachment: extracting attachments: %+v", err)
			return false, errors.Wrap(err, "Error listing volume attachment: extracting attachments")
		}
		for _, va := range list {
			ava := resources.VolumeAttachment{
				ID:       va.ID,
				ServerID: va.ServerID,
				VolumeID: va.VolumeID,
				Device:   va.Device,
			}
			vs = append(vs, ava)
		}
		return true, nil
	})
	if err != nil {
		log.Debugf("Error listing volume attachment: listing attachments: %+v", err)
		return nil, errors.Wrap(err, fmt.Sprintf("Error listing volume types: %s", ProviderErrorToString(err)))
	}
	return vs, nil
}

// DeleteVolumeAttachment deletes the volume attachment identifed by id
func (s *Stack) DeleteVolumeAttachment(serverID, vaID string) error {
	log.Debugf(">>> stacks.openstack::DeleteVolumeAttachment(%s)", serverID)
	defer log.Debugf("<<< stacks.openstack::DeleteVolumeAttachment(%s)", serverID)

	if s == nil {
		panic("Calling stacks.openstack::DeleteVolumeAttachment() from nil pointer!")
	}

	r := volumeattach.Delete(s.ComputeClient, serverID, vaID)
	err := r.ExtractErr()
	if err != nil {
		log.Debugf("Error deleting volume attachment: deleting attachments: %+v", err)
		return errors.Wrap(err, fmt.Sprintf("Error deleting volume attachment '%s': %s", vaID, ProviderErrorToString(err)))
	}
	return nil
}