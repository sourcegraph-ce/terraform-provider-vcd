package vcd

import (
	"fmt"
	log "github.com/sourcegraph-ce/logrus"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/vmware/go-vcloud-director/v2/govcd"
	"github.com/vmware/go-vcloud-director/v2/types/v56"
)

func resourceVcdIndependentDisk() *schema.Resource {
	return &schema.Resource{
		Create: resourceVcdIndependentDiskCreate,
		Read:   resourceVcdIndependentDiskRead,
		Delete: resourceVcdIndependentDiskDelete,
		Importer: &schema.ResourceImporter{
			State: resourceVcdIndependentDiskImport,
		},
		Schema: map[string]*schema.Schema{
			"org": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Description: "The name of organization to use, optional if defined at provider " +
					"level. Useful when connected as sysadmin working across different organizations",
			},
			"vdc": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "The name of VDC to use, optional if defined at provider level",
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"description": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "independent disk description",
			},
			"storage_profile": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			"size": {
				Type:     schema.TypeFloat,
				Required: true,
				ForceNew: true,
				// we enable this when when we solve https://github.com/terraform-providers/terraform-provider-vcd/issues/355
				//ConflictsWith: []string{"size_in_bytes"},
				//Deprecated:    "In favor of size_in_bytes",
				Description: "size in MB",
			},
			// we enable this when when we solve https://github.com/terraform-providers/terraform-provider-vcd/issues/355
			/*			"size_in_bytes": {
						Type:          schema.TypeInt,
						Optional:      true,
						Computed:      true,
						ForceNew:      true,
						ConflictsWith: []string{"size"},
						Description:   "size in bytes",
					},*/
			"bus_type": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				Computed:     true,
				ValidateFunc: validateBusType,
			},
			"bus_sub_type": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				Computed:     true,
				ValidateFunc: validateBusSubType,
			},
			"iops": &schema.Schema{
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "IOPS request for the created disk",
			},
			"owner_name": &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The owner name of the disk",
			},
			"datastore_name": &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Datastore name",
			},
			"is_attached": &schema.Schema{
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "True if the disk is already attached",
			},
		},
	}
}

var busTypes = map[string]string{
	"IDE":  "5",
	"SCSI": "6",
	"SATA": "20",
}
var busTypesFromValues = map[string]string{
	"5":  "IDE",
	"6":  "SCSI",
	"20": "SATA",
}

var busSubTypes = map[string]string{
	"ide":         "IDE",
	"buslogic":    "buslogic",
	"lsilogic":    "lsilogic",
	"lsilogicsas": "lsilogicsas",
	"virtualscsi": "VirtualSCSI",
	"ahci":        "vmware.sata.ahci",
}

var busSubTypesFromValues = map[string]string{
	"ide":              "IDE",
	"buslogic":         "buslogic",
	"lsilogic":         "lsilogic",
	"lsilogicsas":      "lsilogicsas",
	"VirtualSCSI":      "VirtualSCSI",
	"vmware.sata.ahci": "ahci",
}

func resourceVcdIndependentDiskCreate(d *schema.ResourceData, meta interface{}) error {
	vcdClient := meta.(*VCDClient)

	size, sizeProvided := d.GetOk("size")
	//sizeInBytes, sizeInBytesProvided := d.GetOk("size_in_bytes")

	// we enable this when when we solve https://github.com/terraform-providers/terraform-provider-vcd/issues/355
	/*	if !sizeProvided && !sizeInBytesProvided {
		return fmt.Errorf("size_in_bytes isn't provided")
	}*/

	_, vdc, err := vcdClient.GetOrgAndVdcFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrgAndVdc, err)
	}

	diskName := d.Get("name").(string)
	diskRecord, _ := vdc.QueryDisk(diskName)

	if diskRecord != (govcd.DiskRecord{}) {
		return fmt.Errorf("disk with such name already exist : %s", diskName)
	}

	var diskCreateParams *types.DiskCreateParams
	if sizeProvided {
		diskCreateParams = &types.DiskCreateParams{Disk: &types.Disk{
			Name: diskName,
			Size: int64(size.(float64) * 1024 * 1024),
		}}
	}
	/*	if sizeInBytesProvided {
		diskCreateParams = &types.DiskCreateParams{Disk: &types.Disk{
			Name: diskName,
			Size: int64(sizeInBytes.(int)),
		}}
	}*/

	var storageReference types.Reference
	storageProfileValue := d.Get("storage_profile").(string)

	if storageProfileValue != "" {
		storageReference, err = vdc.FindStorageProfileReference(storageProfileValue)
		if err != nil {
			return fmt.Errorf("error finding storage profile %s", storageProfileValue)
		}
		diskCreateParams.Disk.StorageProfile = &types.Reference{HREF: storageReference.HREF}
	}

	busTypeValue := d.Get("bus_type").(string)
	if busTypeValue != "" {
		diskCreateParams.Disk.BusType = busTypes[strings.ToUpper(busTypeValue)]
	}

	busSubTypeValue := d.Get("bus_sub_type").(string)
	if busSubTypeValue != "" {
		diskCreateParams.Disk.BusSubType = busSubTypes[strings.ToLower(busSubTypeValue)]
	}

	diskCreateParams.Disk.Description = d.Get("description").(string)

	task, err := vdc.CreateDisk(diskCreateParams)
	if err != nil {
		return fmt.Errorf("error creating independent disk: %s", err)
	}

	err = task.WaitTaskCompletion()
	if err != nil {
		return fmt.Errorf("error waiting to finish creation of independent disk: %s", err)
	}

	diskHref := task.Task.Owner.HREF
	disk, err := vdc.GetDiskByHref(diskHref)
	if err != nil {
		return fmt.Errorf("unable to find disk with href %s: %s", diskHref, err)
	}

	d.SetId(disk.Disk.Id)

	return resourceVcdIndependentDiskRead(d, meta)
}

func resourceVcdIndependentDiskRead(d *schema.ResourceData, meta interface{}) error {
	vcdClient := meta.(*VCDClient)

	_, vdc, err := vcdClient.GetOrgAndVdcFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrgAndVdc, err)
	}

	identifier := d.Id()
	var disk *govcd.Disk
	if identifier != "" {
		disk, err = vdc.GetDiskById(identifier, true)
		if govcd.IsNotFound(err) {
			log.Printf("unable to find disk with ID %s: %s. Removing from state", identifier, err)
			d.SetId("")
			return nil
		}
		if err != nil {
			return fmt.Errorf("unable to find disk with ID %s: %s", identifier, err)
		}
	} else {
		identifier = d.Get("name").(string)
		disks, err := vdc.GetDisksByName(identifier, true)
		if govcd.IsNotFound(err) {
			log.Printf("unable to find disk with name %s: %s. Removing from state", identifier, err)
			d.SetId("")
			return nil
		}
		if err != nil {
			return fmt.Errorf("unable to find disk with name %s: %s", identifier, err)
		}
		if len(*disks) > 1 {
			return fmt.Errorf("found more than one disk with name %s: %s", identifier, err)
		}
		disk = &(*disks)[0]
	}

	diskRecords, err := vdc.QueryDisks(disk.Disk.Name)
	if err != nil {
		return fmt.Errorf("unable to query disk with name %s: %s", identifier, err)
	}

	var diskRecord *types.DiskRecordType
	for _, entity := range *diskRecords {
		if entity.HREF == disk.Disk.HREF {
			diskRecord = entity
		}
	}

	if diskRecord == nil {
		return fmt.Errorf("unable to find queried disk with name %s: and href: %s, %s", identifier, disk.Disk.HREF, err)
	}

	setMainData(d, disk)
	_ = d.Set("datastore_name", diskRecord.DataStoreName)
	_ = d.Set("is_attached", diskRecord.IsAttached)

	log.Printf("[TRACE] Disk read completed.")
	return nil
}

func setMainData(d *schema.ResourceData, disk *govcd.Disk) {
	d.SetId(disk.Disk.Id)
	_ = d.Set("name", disk.Disk.Name)
	_ = d.Set("description", disk.Disk.Description)
	_ = d.Set("storage_profile", disk.Disk.StorageProfile.Name)
	// we enable this when when we solve https://github.com/terraform-providers/terraform-provider-vcd/issues/355
	//_ = d.Set("size_in_bytes", disk.Disk.Size)
	_ = d.Set("bus_type", busTypesFromValues[disk.Disk.BusType])
	_ = d.Set("bus_sub_type", busSubTypesFromValues[disk.Disk.BusSubType])
	_ = d.Set("iops", disk.Disk.Iops)
	_ = d.Set("owner_name", disk.Disk.Owner.User.Name)
}

func resourceVcdIndependentDiskDelete(d *schema.ResourceData, meta interface{}) error {
	vcdClient := meta.(*VCDClient)

	_, vdc, err := vcdClient.GetOrgAndVdcFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrgAndVdc, err)
	}

	diskRecord, err := vdc.QueryDisk(d.Get("name").(string))
	if err != nil {
		d.SetId("")
		return fmt.Errorf("error finding disk : %#v", err)
	}

	if diskRecord.Disk.IsAttached {
		return fmt.Errorf("can not remove disk as it is attached to vm")
	}

	disk, err := vdc.GetDiskByHref(diskRecord.Disk.HREF)
	if err != nil {
		d.SetId("")
		return fmt.Errorf("error getting disk : %#v", err)
	}

	task, err := disk.Delete()
	if err != nil {
		d.SetId("")
		return fmt.Errorf("error deleting disk : %#v", err)
	}

	err = task.WaitTaskCompletion()
	if err != nil {
		d.SetId("")
		return fmt.Errorf("error waiting for deleting disk : %#v", err)
	}

	return nil
}

var errHelpDiskImport = fmt.Errorf(`resource id must be specified in one of these formats:
'org-name.vdc-name.my-independent-disk-id' to import by rule id
'list@org-name.vdc-name.my-independent-disk-name' to get a list of disks with their IDs`)

// resourceVcdIndependentDiskImport is responsible for importing the resource.
// The following steps happen as part of import
// 1. The user supplies `terraform import _resource_name_ _the_id_string_` command
// 2a. If the `_the_id_string_` contains a dot formatted path to resource as in the example below
// it will try to import it. If it is found - the ID is set
// 2b. If the `_the_id_string_` starts with `list@` and contains path to disk name similar to
// `list@org-name.vdc-name.my-independent-disk-name` then the function lists all independent disks and their IDs in that vdc
// 3. The functions splits the dot-formatted path and tries to lookup the object
// 4. If the lookup succeeds it sets the ID field for `_resource_name_` resource in statefile
// (the resource must be already defined in .tf config otherwise `terraform import` will complain)
// 5. `terraform refresh` is being implicitly launched. The Read method looks up all other fields
// based on the known ID of object.
//
// Example resource name (_resource_name_): vcd_independent_disk.my-disk
// Example import path (_the_id_string_): org-name.vdc-name.my-independent-disk-id
// Example list path (_the_id_string_): list@org-name.vdc-name.my-independent-disk-name
func resourceVcdIndependentDiskImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	var commandOrgName, orgName, vdcName, diskName, diskId string

	resourceURI := strings.Split(d.Id(), ImportSeparator)

	log.Printf("[DEBUG] importing vcd_independent_disk resource with provided id %s", d.Id())

	if len(resourceURI) != 3 {
		return nil, errHelpDiskImport
	}

	if strings.Contains(d.Id(), "list@") {
		commandOrgName, vdcName, diskName = resourceURI[0], resourceURI[1], resourceURI[2]
		commandOrgNameSplit := strings.Split(commandOrgName, "@")
		if len(commandOrgNameSplit) != 2 {
			return nil, errHelpDiskImport
		}
		orgName = commandOrgNameSplit[1]
		return listDisksForImport(meta, orgName, vdcName, diskName)
	} else {
		orgName, vdcName, diskId = resourceURI[0], resourceURI[1], resourceURI[2]
		return getDiskForImport(d, meta, orgName, vdcName, diskId)
	}
}

func getDiskForImport(d *schema.ResourceData, meta interface{}, orgName, vdcName, diskId string) ([]*schema.ResourceData, error) {
	vcdClient := meta.(*VCDClient)
	_, vdc, err := vcdClient.GetOrgAndVdc(orgName, vdcName)
	if err != nil {
		return nil, fmt.Errorf("[independent disk import] unable to find VDC %s: %s ", vdcName, err)
	}

	disk, err := vdc.GetDiskById(diskId, false)
	if err != nil {
		return []*schema.ResourceData{}, fmt.Errorf("unable to find independent disk with id %s: %s",
			d.Id(), err)
	}

	d.SetId(disk.Disk.Id)
	_ = d.Set("name", disk.Disk.Name)
	return []*schema.ResourceData{d}, nil
}

func listDisksForImport(meta interface{}, orgName, vdcName, diskName string) ([]*schema.ResourceData, error) {

	vcdClient := meta.(*VCDClient)
	_, vdc, err := vcdClient.GetOrgAndVdc(orgName, vdcName)
	if err != nil {
		return nil, fmt.Errorf("[independent disk import] unable to find VDC %s: %s ", vdcName, err)
	}

	_, _ = fmt.Fprintln(getTerraformStdout(), "Retrieving all disks by name")
	disks, err := vdc.GetDisksByName(diskName, false)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve disks by name: %s", err)
	}

	writer := tabwriter.NewWriter(getTerraformStdout(), 0, 8, 1, '\t', tabwriter.AlignRight)

	fmt.Fprintln(writer, "No\tID\tName\tDescription\tSize")
	fmt.Fprintln(writer, "--\t--\t----\t------\t----")
	for index, disk := range *disks {
		fmt.Fprintf(writer, "%d\t%s\t%s\t%s\t%d\n", (index + 1), disk.Disk.Id, disk.Disk.Name, disk.Disk.Description, disk.Disk.Size)
	}
	writer.Flush()

	return nil, fmt.Errorf("resource was not imported! %s", errHelpDiskImport)
}
