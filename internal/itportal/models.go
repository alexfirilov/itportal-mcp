package itportal

// ---- Reference / embedded types ----

type CompanyReference struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type SiteReference struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type ContactReference struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type DocumentReference struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type DeviceReference struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type FacilityReference struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type CabinetReference struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type IPNetworkReference struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type SwitchPortReference struct {
	ID int `json:"id,omitempty"`
}

type ContactEmailReference struct {
	ID    int    `json:"id,omitempty"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// UserReference is used for reviewBy fields.
type UserReference struct {
	ID      int                    `json:"id,omitempty"`
	Contact *ContactEmailReference `json:"contact,omitempty"`
}

// TypeItem is a generic id/name pair used for types and categories.
type TypeItem struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// IPRef represents an IP address record used inside IPNetwork.
type IPRef struct {
	ID int    `json:"id,omitempty"`
	IP string `json:"ip,omitempty"`
}

// ---- Address ----

type Address struct {
	ID       int    `json:"id,omitempty"`
	Address1 string `json:"address1,omitempty"`
	Address2 string `json:"address2,omitempty"`
	City     string `json:"city,omitempty"`
	State    string `json:"state,omitempty"`
	Zip      string `json:"zip,omitempty"`
	Country  string `json:"country,omitempty"`
}

// ---- Company ----

type Company struct {
	ID                    int               `json:"id,omitempty"`
	ParentCompany         *CompanyReference `json:"parentCompany,omitempty"`
	Name                  string            `json:"name,omitempty"`
	Site                  *SiteReference    `json:"site,omitempty"`
	Address               *Address          `json:"address,omitempty"`
	StartDate             string            `json:"startDate,omitempty"`
	Description           string            `json:"description,omitempty"`
	Abbreviation          string            `json:"abbreviation,omitempty"`
	WebSite               string            `json:"webSite,omitempty"`
	Status                string            `json:"status,omitempty"`
	Notes                 string            `json:"notes,omitempty"`
	NotesHtml             bool              `json:"notesHtml,omitempty"`
	RemoteAccessNotes     string            `json:"remoteAccessNotes,omitempty"`
	RemoteAccessNotesHtml bool              `json:"remoteAccessNotesHtml,omitempty"`
	ForeignID             int               `json:"foreignId,omitempty"`
	InOut                 *bool             `json:"inOut,omitempty"`
	InOutNotes            string            `json:"inOutNotes,omitempty"`
	Modified              string            `json:"modified,omitempty"`
	URL                   string            `json:"url,omitempty"`
}

// ---- Site ----

type Site struct {
	ID          int               `json:"id,omitempty"`
	Name        string            `json:"name,omitempty"`
	Company     *CompanyReference `json:"company,omitempty"`
	Description string            `json:"description,omitempty"`
	Contact     *ContactReference `json:"contact,omitempty"`
	Diagram     *DocumentReference `json:"diagram,omitempty"`
	NumberOfPCs int               `json:"numberOfPCs,omitempty"`
	ReviewBy    *UserReference    `json:"reviewBy,omitempty"`
	DueDate     string            `json:"dueDate,omitempty"`
	Address     *Address          `json:"address,omitempty"`
	ForeignID   int               `json:"foreignId,omitempty"`
	ForeignType string            `json:"foreignType,omitempty"`
	InOut       *bool             `json:"inOut,omitempty"`
	InOutNotes  string            `json:"inOutNotes,omitempty"`
	Modified    string            `json:"modified,omitempty"`
	URL         string            `json:"url,omitempty"`
}

// ---- Device ----

type Device struct {
	ID              int               `json:"id,omitempty"`
	Name            string            `json:"name,omitempty"`
	Company         *CompanyReference `json:"company,omitempty"`
	Site            *SiteReference    `json:"site,omitempty"`
	Cabinet         *CabinetReference `json:"cabinet,omitempty"`
	Facility        *FacilityReference `json:"facility,omitempty"`
	Type            *TypeItem         `json:"type,omitempty"`
	Description     string            `json:"description,omitempty"`
	Location        string            `json:"location,omitempty"`
	Domain          string            `json:"domain,omitempty"`
	InstallDate     string            `json:"installDate,omitempty"`
	WarrantyExpires string            `json:"warrantyExpires,omitempty"`
	PurchaseDate    string            `json:"purchaseDate,omitempty"`
	RetireDate      string            `json:"retireDate,omitempty"`
	LeaseEndDate    string            `json:"leaseEndDate,omitempty"`
	Manufacturer    string            `json:"manufacturer,omitempty"`
	Model           string            `json:"model,omitempty"`
	IMEI            string            `json:"imei,omitempty"`
	Serial          string            `json:"serial,omitempty"`
	Tag             string            `json:"tag,omitempty"`
	NumberCPU       int               `json:"numberCpu,omitempty"`
	NumberCores     int               `json:"numberCores,omitempty"`
	PurchasePrice   float64           `json:"purchasePrice,omitempty"`
	ReviewBy        *UserReference    `json:"reviewBy,omitempty"`
	DueDate         string            `json:"dueDate,omitempty"`
	ForeignID       int               `json:"foreignId,omitempty"`
	ForeignType     string            `json:"foreignType,omitempty"`
	InOut           *bool             `json:"inOut,omitempty"`
	InOutNotes      string            `json:"inOutNotes,omitempty"`
	Modified        string            `json:"modified,omitempty"`
	URL             string            `json:"url,omitempty"`
}

// DeviceIP represents an IP address assigned to a device.
type DeviceIP struct {
	ID          int                 `json:"id,omitempty"`
	IP          string              `json:"ip,omitempty"`
	MAC         string              `json:"mac,omitempty"`
	Description string              `json:"description,omitempty"`
	IPNetwork   *IPNetworkReference `json:"ipNetwork,omitempty"`
	SwitchPort  *SwitchPortReference `json:"switchPort,omitempty"`
}

// DeviceNote represents a timestamped note on a device.
type DeviceNote struct {
	ID          int    `json:"id,omitempty"`
	Notes       string `json:"notes,omitempty"`
	NotesHtml   bool   `json:"notesHtml,omitempty"`
	DateTime    string `json:"datetime,omitempty"`
	Description string `json:"description,omitempty"`
}

// DeviceMUrl represents a management URL for a device.
type DeviceMUrl struct {
	ID    int    `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
	Notes string `json:"notes,omitempty"`
}

// Credential represents a username/password pair associated with an account or device.
type Credential struct {
	ID          int    `json:"id,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Description string `json:"description,omitempty"`
	Domain      string `json:"domain,omitempty"`
	TwoFACode   string `json:"2faCode,omitempty"`
}

// AdditionalCredential is a portal-level credential record (not bound to a device/account).
type AdditionalCredential struct {
	ID          int    `json:"id,omitempty"`
	URL         string `json:"url,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Description string `json:"description,omitempty"`
}

// ---- Knowledge Base ----

type KBCategory struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type KB struct {
	ID          int               `json:"id,omitempty"`
	Name        string            `json:"name,omitempty"`
	Company     *CompanyReference `json:"company,omitempty"`
	Description string            `json:"description,omitempty"`
	URIPath     string            `json:"uriPath,omitempty"`
	Public      bool              `json:"public,omitempty"`
	Expires     string            `json:"expires,omitempty"`
	Category    *KBCategory       `json:"category,omitempty"`
	ReviewBy    *UserReference    `json:"reviewBy,omitempty"`
	DueDate     string            `json:"dueDate,omitempty"`
	InOut       *bool             `json:"inOut,omitempty"`
	InOutNotes  string            `json:"inOutNotes,omitempty"`
	Modified    string            `json:"modified,omitempty"`
	URL         string            `json:"url,omitempty"`
}

// ---- Contact ----

type ContactType struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Contact struct {
	ID            int               `json:"id,omitempty"`
	Company       *CompanyReference `json:"company,omitempty"`
	FirstName     string            `json:"firstName,omitempty"`
	MiddleInitial string            `json:"middleInitial,omitempty"`
	LastName      string            `json:"lastName,omitempty"`
	Type          *ContactType      `json:"type,omitempty"`
	Site          *SiteReference    `json:"site,omitempty"`
	Email         string            `json:"email,omitempty"`
	DirectNumber  string            `json:"directNumber,omitempty"`
	Extension     string            `json:"extension,omitempty"`
	DirectFax     string            `json:"directFax,omitempty"`
	HomePhone     string            `json:"homePhone,omitempty"`
	Mobile        string            `json:"mobile,omitempty"`
	Notes         string            `json:"notes,omitempty"`
	Modified      string            `json:"modified,omitempty"`
	URL           string            `json:"url,omitempty"`
}

// ---- Account ----

type AccountType struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Account struct {
	ID             int               `json:"id,omitempty"`
	Company        *CompanyReference `json:"company,omitempty"`
	Type           *AccountType      `json:"type,omitempty"`
	Username       string            `json:"username,omitempty"`
	Password       string            `json:"password,omitempty"`
	TwoFACode      string            `json:"2faCode,omitempty"`
	AccountNumber  string            `json:"accountNumber,omitempty"`
	Expires        string            `json:"expires,omitempty"`
	Description    string            `json:"description,omitempty"`
	Email          string            `json:"email,omitempty"`
	Representative string            `json:"representative,omitempty"`
	AccountURL     string            `json:"accountUrl,omitempty"`
	TechTelephone  string            `json:"techTelephone,omitempty"`
	SalesTelephone string            `json:"salesTelephone,omitempty"`
	ReviewBy       *UserReference    `json:"reviewBy,omitempty"`
	DueDate        string            `json:"dueDate,omitempty"`
	Notes          string            `json:"notes,omitempty"`
	Modified       string            `json:"modified,omitempty"`
	URL            string            `json:"url,omitempty"`
}

// ---- Agreement ----

type AgreementType struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Agreement struct {
	ID          int               `json:"id,omitempty"`
	Company     *CompanyReference `json:"company,omitempty"`
	Type        *AgreementType    `json:"type,omitempty"`
	Description string            `json:"description,omitempty"`
	Vendor      string            `json:"vendor,omitempty"`
	Site        *SiteReference    `json:"site,omitempty"`
	Contact     *ContactReference `json:"contact,omitempty"`
	Count       int               `json:"count,omitempty"`
	Cost        float64           `json:"cost,omitempty"`
	DateIssued  string            `json:"dateIssued,omitempty"`
	DateExpires string            `json:"dateExpires,omitempty"`
	SerialNumber string           `json:"serialNumber,omitempty"`
	InstallDate string            `json:"installDate,omitempty"`
	ReviewBy    *UserReference    `json:"reviewBy,omitempty"`
	DueDate     string            `json:"dueDate,omitempty"`
	ForeignID   int               `json:"foreignId,omitempty"`
	ForeignType string            `json:"foreignType,omitempty"`
	Notes       string            `json:"notes,omitempty"`
	Modified    string            `json:"modified,omitempty"`
	URL         string            `json:"url,omitempty"`
}

// ---- Document ----

type DocumentType struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Document struct {
	ID          int               `json:"id,omitempty"`
	Company     *CompanyReference `json:"company,omitempty"`
	Type        *DocumentType     `json:"type,omitempty"`
	Description string            `json:"description,omitempty"`
	URLLink     string            `json:"urlLink,omitempty"`
	Public      bool              `json:"public,omitempty"`
	ReviewBy    *UserReference    `json:"reviewBy,omitempty"`
	DueDate     string            `json:"dueDate,omitempty"`
	InOut       *bool             `json:"inOut,omitempty"`
	InOutNotes  string            `json:"inOutNotes,omitempty"`
	Modified    string            `json:"modified,omitempty"`
	URL         string            `json:"url,omitempty"`
}

// ---- IP Network ----

type IPNetwork struct {
	ID             int               `json:"id,omitempty"`
	Name           string            `json:"name,omitempty"`
	Company        *CompanyReference `json:"company,omitempty"`
	Description    string            `json:"description,omitempty"`
	Site           *SiteReference    `json:"site,omitempty"`
	Network        string            `json:"network,omitempty"`
	SubnetMask     string            `json:"subnetMask,omitempty"`
	DefaultGateway *IPRef            `json:"defaultGateway,omitempty"`
	DNSServer1     *IPRef            `json:"dnsServer1,omitempty"`
	DNSServer2     *IPRef            `json:"dnsServer2,omitempty"`
	DHCPServer     *IPRef            `json:"dhcpServer,omitempty"`
	VlanID         int               `json:"vlanId,omitempty"`
	ReviewBy       *UserReference    `json:"reviewBy,omitempty"`
	DueDate        string            `json:"dueDate,omitempty"`
	Notes          string            `json:"notes,omitempty"`
	Modified       string            `json:"modified,omitempty"`
}

// ---- Facility ----

type FacilityType struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Facility struct {
	ID            int               `json:"id,omitempty"`
	Name          string            `json:"name,omitempty"`
	Company       *CompanyReference `json:"company,omitempty"`
	Type          *FacilityType     `json:"type,omitempty"`
	Site          *SiteReference    `json:"site,omitempty"`
	Description   string            `json:"description,omitempty"`
	NumberOfUsers int               `json:"numberOfUsers,omitempty"`
	Diagram       *DocumentReference `json:"diagram,omitempty"`
	ReviewBy      *UserReference    `json:"reviewBy,omitempty"`
	DueDate       string            `json:"dueDate,omitempty"`
	Address       *Address          `json:"address,omitempty"`
	Notes         string            `json:"notes,omitempty"`
	Modified      string            `json:"modified,omitempty"`
	URL           string            `json:"url,omitempty"`
}

// ---- Cabinet ----

type Cabinet struct {
	ID          int                `json:"id,omitempty"`
	Name        string             `json:"name,omitempty"`
	Company     *CompanyReference  `json:"company,omitempty"`
	Description string             `json:"description,omitempty"`
	Site        *SiteReference     `json:"site,omitempty"`
	Facility    *FacilityReference `json:"facility,omitempty"`
	Contact     *ContactReference  `json:"contact,omitempty"`
	Diagram     *DocumentReference `json:"diagram,omitempty"`
	ReviewBy    *UserReference     `json:"reviewBy,omitempty"`
	DueDate     string             `json:"dueDate,omitempty"`
	Address     *Address           `json:"address,omitempty"`
	Notes       string             `json:"notes,omitempty"`
	Modified    string             `json:"modified,omitempty"`
	URL         string             `json:"url,omitempty"`
}

// ---- Configuration ----

type ConfigurationType struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Configuration struct {
	ID          int               `json:"id,omitempty"`
	Name        string            `json:"name,omitempty"`
	Company     *CompanyReference `json:"company,omitempty"`
	Type        *ConfigurationType `json:"type,omitempty"`
	Device      *DeviceReference  `json:"device,omitempty"`
	URIPath     string            `json:"uriPath,omitempty"`
	InstallDate string            `json:"installDate,omitempty"`
	DateExpires string            `json:"dateExpires,omitempty"`
	ReviewBy    *UserReference    `json:"reviewBy,omitempty"`
	DueDate     string            `json:"dueDate,omitempty"`
	Notes       string            `json:"notes,omitempty"`
	Modified    string            `json:"modified,omitempty"`
	URL         string            `json:"url,omitempty"`
}

// ---- Forms ----

type FormField struct {
	ID    int    `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

type FormSection struct {
	ID     int         `json:"id,omitempty"`
	Name   string      `json:"name,omitempty"`
	Fields []FormField `json:"fields,omitempty"`
}

type FormTemplate struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type FormInstance struct {
	ID       int               `json:"id,omitempty"`
	Company  *CompanyReference `json:"company,omitempty"`
	Form     *FormTemplate     `json:"form,omitempty"`
	Sections []FormSection     `json:"sections,omitempty"`
}

// ---- Templates ----

type TemplateField struct {
	ID    int    `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

type TemplateSection struct {
	ID     int              `json:"id,omitempty"`
	Name   string           `json:"name,omitempty"`
	Fields []*TemplateField `json:"fields,omitempty"`
}

type Template struct {
	ID       int               `json:"id,omitempty"`
	Name     string            `json:"name,omitempty"`
	Sections []*TemplateSection `json:"sections,omitempty"`
	Modified string            `json:"modified,omitempty"`
}

// ---- Interactions ----

type Interaction struct {
	ID       int    `json:"id,omitempty"`
	Notes    string `json:"notes,omitempty"`
	DateTime string `json:"datetime,omitempty"`
}

// ---- System ----

type User struct {
	ID    int    `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

type SecurityGroup struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Country struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Code string `json:"code,omitempty"`
}
