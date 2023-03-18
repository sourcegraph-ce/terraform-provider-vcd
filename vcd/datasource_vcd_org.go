package vcd

import (
	"fmt"
	log "github.com/sourcegraph-ce/logrus"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func datasourceVcdOrg() *schema.Resource {
	return &schema.Resource{
		Read: datasourceVcdOrgRead,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Organization name for lookup",
			},
			"full_name": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"description": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"is_enabled": &schema.Schema{
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "True if this organization is enabled (allows login and all other operations).",
			},
			"deployed_vm_quota": &schema.Schema{
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Maximum number of virtual machines that can be deployed simultaneously by a member of this organization. (0 = unlimited)",
			},
			"stored_vm_quota": &schema.Schema{
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Maximum number of virtual machines in vApps or vApp templates that can be stored in an undeployed state by a member of this organization. (0 = unlimited)",
			},
			"can_publish_catalogs": &schema.Schema{
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "True if this organization is allowed to share catalogs.",
			},
			"vapp_lease": &schema.Schema{
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"maximum_runtime_lease_in_sec": &schema.Schema{
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "How long vApps can run before they are automatically stopped (in seconds)",
						},
						"power_off_on_runtime_lease_expiration": &schema.Schema{
							Type:     schema.TypeBool,
							Computed: true,
							Description: "When true, vApps are powered off when the runtime lease expires. " +
								"When false, vApps are suspended when the runtime lease expires",
						},
						"maximum_storage_lease_in_sec": &schema.Schema{
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "How long stopped vApps are available before being automatically cleaned up (in seconds)",
						},
						"delete_on_storage_lease_expiration": &schema.Schema{
							Type:     schema.TypeBool,
							Computed: true,
							Description: "If true, storage for a vApp is deleted when the vApp's lease expires. " +
								"If false, the storage is flagged for deletion, but not deleted.",
						},
					},
				},
			},
			"vapp_template_lease": &schema.Schema{
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"maximum_storage_lease_in_sec": &schema.Schema{
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "How long vApp templates are available before being automatically cleaned up (in seconds)",
						},
						"delete_on_storage_lease_expiration": &schema.Schema{
							Type:     schema.TypeBool,
							Computed: true,
							Description: "If true, storage for a vAppTemplate is deleted when the vAppTemplate lease expires. " +
								"If false, the storage is flagged for deletion, but not deleted",
						},
					},
				},
			},
			"delay_after_power_on_seconds": &schema.Schema{
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Specifies this organization's default for virtual machine boot delay after power on.",
			},
		},
	}
}

func datasourceVcdOrgRead(d *schema.ResourceData, meta interface{}) error {
	vcdClient := meta.(*VCDClient)

	identifier := d.Get("name").(string)
	log.Printf("Reading Org with id %s", identifier)
	adminOrg, err := vcdClient.VCDClient.GetAdminOrgByNameOrId(identifier)

	if err != nil {
		log.Printf("Org with id %s not found. Setting ID to nothing", identifier)
		d.SetId("")
		return fmt.Errorf("org %s not found: %s", identifier, err)
	}
	log.Printf("Org with id %s found", identifier)
	d.SetId(adminOrg.AdminOrg.ID)
	return setOrgData(d, adminOrg)
}
