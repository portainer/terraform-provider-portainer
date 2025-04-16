package internal

import (
	"fmt"
	"io"

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
		Create: resourceOpenAMTCreate,
		Read:   resourceOpenAMTRead,
		Delete: resourceOpenAMTDelete,
		Update: nil,
		Schema: map[string]*schema.Schema{
			"cert_file_content":  {Type: schema.TypeString, Required: true, ForceNew: true, Sensitive: true},
			"cert_file_name":     {Type: schema.TypeString, Required: true, ForceNew: true},
			"cert_file_password": {Type: schema.TypeString, Required: true, ForceNew: true, Sensitive: true},
			"domain_name":        {Type: schema.TypeString, Required: true, ForceNew: true},
			"enabled":            {Type: schema.TypeBool, Required: true, ForceNew: true},
			"mpspassword":        {Type: schema.TypeString, Required: true, ForceNew: true, Sensitive: true},
			"mpsserver":          {Type: schema.TypeString, Required: true, ForceNew: true},
			"mpsuser":            {Type: schema.TypeString, Required: true, ForceNew: true},
		},
	}
}

func resourceOpenAMTCreate(d *schema.ResourceData, meta interface{}) error {
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

	url := fmt.Sprintf("%s/open_amt", client.Endpoint)

	resp, err := client.DoRequest("POST", url, nil, settings)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to enable OpenAMT: %s", string(body))
	}

	d.SetId("openamt-enabled")
	return nil
}

func resourceOpenAMTRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceOpenAMTDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
