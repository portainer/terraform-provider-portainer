package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type OpenAMTSettings struct {
	CertFileContent  string `json:"certFileContent"`
	CertFileName     string `json:"certFileName"`
	CertFilePassword string `json:"certFilePassword"`
	DomainName       string `json:"domainName"`
	Enabled          bool   `json:"enabled"`
	MpsPassword      string `json:"mpspassword"`
	MpsServer        string `json:"mpsserver"`
	MpsUser          string `json:"mpsuser"`
}

func resourceOpenAMT() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceOpenAMTCreate,
		ReadContext:   resourceOpenAMTRead,
		DeleteContext: resourceOpenAMTDelete,
		UpdateContext: nil,
		Schema: map[string]*schema.Schema{
			"cert_file_content":  {Type: schema.TypeString, Required: true, ForceNew: true, Sensitive: true, Description: "Sensitive base64-encoded content of the OpenAMT provisioning certificate file."},
			"cert_file_name":     {Type: schema.TypeString, Required: true, ForceNew: true, Description: "File name of the OpenAMT provisioning certificate."},
			"cert_file_password": {Type: schema.TypeString, Required: true, ForceNew: true, Sensitive: true, Description: "Sensitive password protecting the OpenAMT provisioning certificate."},
			"domain_name":        {Type: schema.TypeString, Required: true, ForceNew: true, Description: "Domain name configured for OpenAMT provisioning."},
			"enabled":            {Type: schema.TypeBool, Required: true, ForceNew: true, Description: "Whether the OpenAMT integration is enabled in Portainer."},
			"mpspassword":        {Type: schema.TypeString, Required: true, ForceNew: true, Sensitive: true, Description: "Sensitive password used to authenticate against the OpenAMT MPS (Management Presence Server)."},
			"mpsserver":          {Type: schema.TypeString, Required: true, ForceNew: true, Description: "Hostname or URL of the OpenAMT MPS (Management Presence Server)."},
			"mpsuser":            {Type: schema.TypeString, Required: true, ForceNew: true, Description: "Username used to authenticate against the OpenAMT MPS (Management Presence Server)."},
		},
	}
}

func resourceOpenAMTCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	settings := OpenAMTSettings{
		CertFileContent:  d.Get("cert_file_content").(string),
		CertFileName:     d.Get("cert_file_name").(string),
		CertFilePassword: d.Get("cert_file_password").(string),
		DomainName:       d.Get("domain_name").(string),
		Enabled:          d.Get("enabled").(bool),
		MpsPassword:      d.Get("mpspassword").(string),
		MpsServer:        d.Get("mpsserver").(string),
		MpsUser:          d.Get("mpsuser").(string),
	}

	resp, err := client.DoRequest("POST", "/open_amt", nil, settings)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to enable OpenAMT: %s", string(body)))
	}

	d.SetId("openamt-enabled")
	return nil
}

func resourceOpenAMTRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceOpenAMTDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
