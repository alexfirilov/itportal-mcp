# Portal API 2.1 - Comprehensive Test Script
# Tests all API 2.1 endpoints: GET, POST, PATCH, DELETE operations

param(
    [string]$ApiHost = "",
    [string]$ApiKey = "",
    [string]$EncryptionKey = ""
)

# Hardcoded defaults (set these to avoid prompts)
$DefaultApiHost = ""
$DefaultApiKey = ""
$DefaultEncryptionKey = ""

# Use parameter, then hardcoded default, then prompt
if ($ApiHost -eq "") {
    if ($DefaultApiHost -ne "") {
        $ApiHost = $DefaultApiHost
    } else {
        $ApiHost = Read-Host "Enter API Host (e.g., http://localhost)"
    }
}

if ($ApiKey -eq "") {
    if ($DefaultApiKey -ne "") {
        $ApiKey = $DefaultApiKey
    } else {
        $ApiKey = Read-Host "Enter API Key"
    }
}

if ($EncryptionKey -eq "") {
    if ($DefaultEncryptionKey -ne "") {
        $EncryptionKey = $DefaultEncryptionKey
    } else {
        $EncryptionKey = Read-Host "Enter Encryption Key (leave blank if org does not use custom encryption)"
    }
}

# Validate inputs
if ($ApiHost -eq "" -or $ApiKey -eq "") {
    Write-Host "Error: ApiHost and ApiKey are required." -ForegroundColor Red
    exit 1
}

# Load System.Web for potential URL encoding
Add-Type -AssemblyName System.Web

# Build Basic Auth header (API expects key as password in Basic auth)
$BasicAuth = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes(":$ApiKey"))

# Common headers
$Headers = @{
    "Content-Type" = "application/json; charset=utf-8"
    "Authorization" = "Basic $BasicAuth"
}

# Headers with encryption key (used for credential endpoints on custom-encryption orgs)
$CredentialHeaders = @{
    "Content-Type" = "application/json; charset=utf-8"
    "Authorization" = "Basic $BasicAuth"
}
if ($EncryptionKey -ne "") {
    $CredentialHeaders["X-Encryption-Key"] = $EncryptionKey
}

# Patch headers with encryption key
$PatchCredentialHeaders = @{
    "Content-Type" = "application/merge-patch+json"
    "Authorization" = "Basic $BasicAuth"
}
if ($EncryptionKey -ne "") {
    $PatchCredentialHeaders["X-Encryption-Key"] = $EncryptionKey
}

# Test counters
$script:passCount = 0
$script:failCount = 0
$script:skipCount = 0

# Track all created entities for cleanup
$script:createdCompanyId = $null
$script:createdAccountId = $null
$script:createdContactId = $null
$script:createdSiteId = $null
$script:createdAddressId = $null
$script:createdTopLevelAddressId = $null
$script:createdDeviceId = $null
$script:createdDeviceIpId = $null
$script:createdDeviceMurlId = $null
$script:createdDeviceNoteId = $null
$script:createdDocumentId = $null
$script:createdKbId = $null
$script:createdConfigurationId = $null
$script:createdFacilityId = $null
$script:createdAgreementId = $null
$script:createdCabinetId = $null
$script:createdIpNetworkId = $null
$script:createdAdditionalCredentialId = $null
$script:createdInteractionId = $null

# Store types for creating entities
$script:companyTypeId = $null
$script:accountTypeId = $null
$script:contactTypeId = $null
$script:deviceTypeId = $null
$script:documentTypeId = $null
$script:facilityTypeId = $null
$script:agreementTypeId = $null
$script:configurationTypeId = $null
$script:kbCategoryId = $null
$script:kbSubCategoryId = $null

# ============================================================
# HELPER FUNCTIONS
# ============================================================

function Write-TestResult {
    param($TestName, $Success, $Details = "")
    if ($Success) {
        Write-Host "[PASS] $TestName" -ForegroundColor Green
        $script:passCount++
    } else {
        Write-Host "[FAIL] $TestName" -ForegroundColor Red
        $script:failCount++
    }
    if ($Details) {
        Write-Host "       $Details" -ForegroundColor Gray
    }
}

function Write-Skip {
    param($TestName, $Reason = "")
    Write-Host "[SKIP] $TestName" -ForegroundColor Yellow
    if ($Reason) {
        Write-Host "       $Reason" -ForegroundColor Gray
    }
    $script:skipCount++
}

function Test-GetEndpoint {
    param($Url, $TestName, [switch]$Expect404, $UseHeaders = $Headers)

    try {
        $response = Invoke-RestMethod -Uri $Url -Method Get -Headers $UseHeaders
        if ($Expect404) {
            Write-TestResult $TestName $false "Expected 404, got success"
            return $null
        }
        Write-TestResult $TestName $true "Count: $($response.data.count)"
        return $response
    } catch {
        if ($Expect404 -and $_.Exception.Response.StatusCode -eq 404) {
            Write-TestResult $TestName $true "Correctly returned 404"
            return $null
        }
        Write-TestResult $TestName $false $_.Exception.Message
        return $null
    }
}

function Invoke-ApiRequest {
    param($Method, $Url, $Body = $null)

    $params = @{
        Uri = $Url
        Method = $Method
        Headers = $Headers
        UseBasicParsing = $true
    }

    if ($Body) {
        $params.Body = $Body | ConvertTo-Json -Depth 5 -Compress
    }

    return Invoke-WebRequest @params
}

function Test-PostEndpoint {
    param($Url, $Body, $TestName, $IdPattern, $UseHeaders = $Headers)

    $jsonBody = $Body | ConvertTo-Json -Depth 5 -Compress

    try {
        $response = Invoke-WebRequest -Uri $Url -Method Post -Headers $UseHeaders -Body $jsonBody -UseBasicParsing

        if ($response.StatusCode -eq 201) {
            $location = $response.Headers["Location"]
            $createdId = $null
            if ($location -match $IdPattern) {
                $createdId = $matches[1]
            }
            Write-TestResult $TestName $true "Created ID: $createdId"
            return $createdId
        } else {
            Write-TestResult $TestName $false "Status: $($response.StatusCode)"
            return $null
        }
    } catch {
        $errorMsg = $_.Exception.Message
        if ($_.Exception.Response) {
            try {
                $stream = $_.Exception.Response.GetResponseStream()
                $reader = New-Object System.IO.StreamReader($stream)
                $errorBody = $reader.ReadToEnd()
                $reader.Close()
                if ($errorBody -and $errorBody.Length -lt 200) {
                    $errorMsg = $errorBody
                }
            } catch {}
        }
        Write-TestResult $TestName $false $errorMsg
        return $null
    }
}

function Test-PatchEndpoint {
    param($Url, $Body, $TestName, $UseHeaders = $null)

    $jsonBody = $Body | ConvertTo-Json -Depth 5 -Compress

    # PATCH requires application/merge-patch+json content type
    if ($UseHeaders) {
        $patchHeaders = $UseHeaders
    } else {
        $patchHeaders = @{
            "Content-Type" = "application/merge-patch+json"
            "Authorization" = "Basic $BasicAuth"
        }
    }

    try {
        $response = Invoke-WebRequest -Uri $Url -Method Patch -Headers $patchHeaders -Body $jsonBody -UseBasicParsing
        if ($response.StatusCode -eq 204) {
            Write-TestResult $TestName $true "Updated successfully"
            return $true
        } else {
            Write-TestResult $TestName $false "Status: $($response.StatusCode)"
            return $false
        }
    } catch {
        Write-TestResult $TestName $false $_.Exception.Message
        return $false
    }
}

function Test-DeleteEndpoint {
    param($Url, $TestName)

    try {
        $response = Invoke-WebRequest -Uri $Url -Method Delete -Headers $Headers -UseBasicParsing
        if ($response.StatusCode -eq 204 -or $response.StatusCode -eq 200) {
            Write-TestResult $TestName $true "Deleted successfully"
            return $true
        } else {
            Write-TestResult $TestName $false "Status: $($response.StatusCode)"
            return $false
        }
    } catch {
        Write-TestResult $TestName $false $_.Exception.Message
        return $false
    }
}

function Get-FirstTypeId {
    param($TypeCategory)

    try {
        $response = Invoke-RestMethod -Uri "$ApiHost/api/2.1/types/$TypeCategory/" -Method Get -Headers $Headers
        if ($response.data.results.Count -gt 0) {
            return $response.data.results[0].id
        }
    } catch {}
    return $null
}

# ============================================================
# START TESTS
# ============================================================

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "  Portal API 2.1 - Comprehensive Tests" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Host: $ApiHost" -ForegroundColor Gray
Write-Host "Time: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Gray
if ($EncryptionKey -ne "") {
    Write-Host "Encryption Key: (provided)" -ForegroundColor Gray
} else {
    Write-Host "Encryption Key: (none - org does not use custom encryption)" -ForegroundColor Gray
}

# ============================================================
# SECTION 1: GET TYPES (needed for creating entities)
# ============================================================
Write-Host "`n=== TYPES ===" -ForegroundColor Cyan

$script:companyTypeId = Get-FirstTypeId "company"
Write-Host "  Company Type ID: $($script:companyTypeId)" -ForegroundColor Gray

$script:accountTypeId = Get-FirstTypeId "account"
Write-Host "  Account Type ID: $($script:accountTypeId)" -ForegroundColor Gray

$script:contactTypeId = Get-FirstTypeId "contact"
Write-Host "  Contact Type ID: $($script:contactTypeId)" -ForegroundColor Gray

$script:deviceTypeId = Get-FirstTypeId "device"
Write-Host "  Device Type ID: $($script:deviceTypeId)" -ForegroundColor Gray

$script:documentTypeId = Get-FirstTypeId "document"
Write-Host "  Document Type ID: $($script:documentTypeId)" -ForegroundColor Gray

$script:facilityTypeId = Get-FirstTypeId "facility"
Write-Host "  Facility Type ID: $($script:facilityTypeId)" -ForegroundColor Gray

$script:agreementTypeId = Get-FirstTypeId "agreement"
Write-Host "  Agreement Type ID: $($script:agreementTypeId)" -ForegroundColor Gray

$script:configurationTypeId = Get-FirstTypeId "configuration"
Write-Host "  Configuration Type ID: $($script:configurationTypeId)" -ForegroundColor Gray

# Get KB Categories (different endpoint structure)
try {
    $catResponse = Invoke-RestMethod -Uri "$ApiHost/api/2.1/categories/kb/" -Method Get -Headers $Headers
    if ($catResponse.data.results.Count -gt 0) {
        $script:kbCategoryId = $catResponse.data.results[0].id
        Write-Host "  KB Category ID: $($script:kbCategoryId)" -ForegroundColor Gray
        # Get first subcategory if available
        if ($catResponse.data.results[0].subCategories -and $catResponse.data.results[0].subCategories.Count -gt 0) {
            $script:kbSubCategoryId = $catResponse.data.results[0].subCategories[0].id
            Write-Host "  KB SubCategory ID: $($script:kbSubCategoryId)" -ForegroundColor Gray
        } else {
            Write-Host "  KB SubCategory ID: (none found)" -ForegroundColor Gray
        }
    }
} catch {
    Write-Host "  KB Category ID: (error fetching)" -ForegroundColor Yellow
}

# ============================================================
# SECTION 2: CREATE TEST COMPANY (parent for all entities)
# ============================================================
Write-Host "`n=== COMPANIES ===" -ForegroundColor Cyan

$testCompanyName = "API-Test"

# Check if API-Test company already exists
$existingCompany = $null
try {
    $searchResponse = Invoke-RestMethod -Uri "$ApiHost/api/2.1/companies/?name=$testCompanyName" -Method Get -Headers $Headers
    if ($searchResponse.data.results.Count -gt 0) {
        $existingCompany = $searchResponse.data.results[0]
    }
} catch {}

if ($existingCompany) {
    Write-Host "`n[ERROR] Company '$testCompanyName' already exists (ID: $($existingCompany.id))" -ForegroundColor Red
    Write-Host "        Please delete it first before running the test script." -ForegroundColor Red
    Write-Host "`nExiting...`n" -ForegroundColor Yellow
    exit 1
}

# Create new company
$companyBody = @{
    name = $testCompanyName
    type = @{ id = $script:companyTypeId }
    status = "Active"
    description = "Created via API test script"
    contact = @{ firstName = "Test"; lastName = "Contact" }
}

$script:createdCompanyId = Test-PostEndpoint "$ApiHost/api/2.1/companies/" $companyBody "POST /companies/" "/companies/(\d+)/"

if ($script:createdCompanyId) {
    # Verify GET
    Test-GetEndpoint "$ApiHost/api/2.1/companies/$($script:createdCompanyId)/" "GET /companies/$($script:createdCompanyId)/"

    # Test PATCH
    $patchBody = @{ abbreviation = "APITEST" }
    Test-PatchEndpoint "$ApiHost/api/2.1/companies/$($script:createdCompanyId)/" $patchBody "PATCH /companies/$($script:createdCompanyId)/"
}

# Test GET list with filters
Test-GetEndpoint "$ApiHost/api/2.1/companies/?limit=5" "GET /companies/?limit=5"
Test-GetEndpoint "$ApiHost/api/2.1/companies/?status=Active&limit=3" "GET /companies/?status=Active"

# ITP3-11604: Company-scoped device and document lists
if ($script:createdCompanyId) {
    Test-GetEndpoint "$ApiHost/api/2.1/companies/$($script:createdCompanyId)/devices/" "GET /companies/$($script:createdCompanyId)/devices/"
    Test-GetEndpoint "$ApiHost/api/2.1/companies/$($script:createdCompanyId)/documents/" "GET /companies/$($script:createdCompanyId)/documents/"
} else {
    Write-Skip "GET /companies/{id}/devices/" "No test company created"
    Write-Skip "GET /companies/{id}/documents/" "No test company created"
}

# ============================================================
# SECTION 3: ACCOUNTS
# ============================================================
Write-Host "`n=== ACCOUNTS ===" -ForegroundColor Cyan

if ($script:createdCompanyId) {
    $accountBody = @{
        name = "API-Test Account"
        company = @{ id = [int]$script:createdCompanyId }
        type = @{ id = $script:accountTypeId }
        description = "Test account created by API"
    }

    $script:createdAccountId = Test-PostEndpoint "$ApiHost/api/2.1/accounts/" $accountBody "POST /accounts/" "/accounts/(\d+)/"

    if ($script:createdAccountId) {
        Test-GetEndpoint "$ApiHost/api/2.1/accounts/$($script:createdAccountId)/" "GET /accounts/$($script:createdAccountId)/"

        $patchBody = @{ description = "Updated by API test" }
        Test-PatchEndpoint "$ApiHost/api/2.1/accounts/$($script:createdAccountId)/" $patchBody "PATCH /accounts/$($script:createdAccountId)/"
    }
} else {
    Write-Skip "POST /accounts/" "No test company created"
}

Test-GetEndpoint "$ApiHost/api/2.1/accounts/?limit=5" "GET /accounts/?limit=5"

# ============================================================
# SECTION 4: CONTACTS
# ============================================================
Write-Host "`n=== CONTACTS ===" -ForegroundColor Cyan

if ($script:createdCompanyId) {
    $contactBody = @{
        firstName = "API-Test"
        lastName = "Contact"
        company = @{ id = [int]$script:createdCompanyId }
        type = @{ id = $script:contactTypeId }
    }

    $script:createdContactId = Test-PostEndpoint "$ApiHost/api/2.1/contacts/" $contactBody "POST /contacts/" "/contacts/(\d+)/"

    if ($script:createdContactId) {
        Test-GetEndpoint "$ApiHost/api/2.1/contacts/$($script:createdContactId)/" "GET /contacts/$($script:createdContactId)/"

        $patchBody = @{ description = "API-Test contact description" }
        Test-PatchEndpoint "$ApiHost/api/2.1/contacts/$($script:createdContactId)/" $patchBody "PATCH /contacts/$($script:createdContactId)/"
    }
} else {
    Write-Skip "POST /contacts/" "No test company created"
}

Test-GetEndpoint "$ApiHost/api/2.1/contacts/?limit=5" "GET /contacts/?limit=5"

# ============================================================
# SECTION 5: SITES
# ============================================================
Write-Host "`n=== SITES ===" -ForegroundColor Cyan

if ($script:createdCompanyId) {
    $siteBody = @{
        name = "API-Test Site"
        company = @{ id = [int]$script:createdCompanyId }
    }

    $script:createdSiteId = Test-PostEndpoint "$ApiHost/api/2.1/sites/" $siteBody "POST /sites/" "/sites/(\d+)/"

    if ($script:createdSiteId) {
        Test-GetEndpoint "$ApiHost/api/2.1/sites/$($script:createdSiteId)/" "GET /sites/$($script:createdSiteId)/"

        $patchBody = @{ description = "Updated site" }
        Test-PatchEndpoint "$ApiHost/api/2.1/sites/$($script:createdSiteId)/" $patchBody "PATCH /sites/$($script:createdSiteId)/"
    }
} else {
    Write-Skip "POST /sites/" "No test company created"
}

Test-GetEndpoint "$ApiHost/api/2.1/sites/?limit=5" "GET /sites/?limit=5"

# ============================================================
# SECTION 6: DEVICES
# ============================================================
Write-Host "`n=== DEVICES ===" -ForegroundColor Cyan

if ($script:createdCompanyId -and $script:createdSiteId) {
    $deviceBody = @{
        name = "API-Test Device"
        hostName = "api-test-device.local"
        company = @{ id = [int]$script:createdCompanyId }
        site = @{ id = [int]$script:createdSiteId }
        type = @{ id = $script:deviceTypeId }
    }

    $script:createdDeviceId = Test-PostEndpoint "$ApiHost/api/2.1/devices/" $deviceBody "POST /devices/" "/devices/(\d+)/"

    if ($script:createdDeviceId) {
        Test-GetEndpoint "$ApiHost/api/2.1/devices/$($script:createdDeviceId)/" "GET /devices/$($script:createdDeviceId)/"

        $patchBody = @{ description = "Updated device" }
        Test-PatchEndpoint "$ApiHost/api/2.1/devices/$($script:createdDeviceId)/" $patchBody "PATCH /devices/$($script:createdDeviceId)/"

        # Device sub-entities
        Write-Host "`n--- Device Sub-entities ---" -ForegroundColor Gray

        # Device IPs
        $ipBody = @{ ip = "192.168.1.100"; description = "API-Test IP" }
        $script:createdDeviceIpId = Test-PostEndpoint "$ApiHost/api/2.1/devices/$($script:createdDeviceId)/ips/" $ipBody "POST /devices/{id}/ips/" "/ips/(\d+)/"

        # Device Management URLs (title, url, and preferred are required)
        $murlBody = @{ title = "API-Test Management URL"; url = "https://api-test.example.com"; preferred = $false }
        $script:createdDeviceMurlId = Test-PostEndpoint "$ApiHost/api/2.1/devices/$($script:createdDeviceId)/managementUrls/" $murlBody "POST /devices/{id}/managementUrls/" "/managementUrls/(\d+)/"

        # Device Notes (requires description or notes field)
        $noteBody = @{ description = "API-Test"; notes = "API-Test device note" }
        $script:createdDeviceNoteId = Test-PostEndpoint "$ApiHost/api/2.1/devices/$($script:createdDeviceId)/notes/" $noteBody "POST /devices/{id}/notes/" "/notes/(\d+)/"
    }
} else {
    Write-Skip "POST /devices/" "No test company or site created"
}

Test-GetEndpoint "$ApiHost/api/2.1/devices/?limit=5" "GET /devices/?limit=5"

# ============================================================
# SECTION 7: DOCUMENTS
# ============================================================
Write-Host "`n=== DOCUMENTS ===" -ForegroundColor Cyan

if ($script:createdCompanyId) {
    $documentBody = @{
        name = "API-Test Document"
        company = @{ id = [int]$script:createdCompanyId }
        type = @{ id = $script:documentTypeId }
    }

    $script:createdDocumentId = Test-PostEndpoint "$ApiHost/api/2.1/documents/" $documentBody "POST /documents/" "/documents/(\d+)/"

    if ($script:createdDocumentId) {
        Test-GetEndpoint "$ApiHost/api/2.1/documents/$($script:createdDocumentId)/" "GET /documents/$($script:createdDocumentId)/"

        $patchBody = @{ description = "Updated document" }
        Test-PatchEndpoint "$ApiHost/api/2.1/documents/$($script:createdDocumentId)/" $patchBody "PATCH /documents/$($script:createdDocumentId)/"
    }
} else {
    Write-Skip "POST /documents/" "No test company created"
}

Test-GetEndpoint "$ApiHost/api/2.1/documents/?limit=5" "GET /documents/?limit=5"

# ============================================================
# SECTION 8: KBs
# ============================================================
Write-Host "`n=== KBs ===" -ForegroundColor Cyan

if ($script:kbCategoryId -and $script:kbSubCategoryId -and $script:createdCompanyId) {
    $kbBody = @{
        name = "API-Test KB"
        category = @{ id = $script:kbCategoryId }
        subCategory = @{ id = $script:kbSubCategoryId }
        article = "Test KB article content"
        company = @{ id = [int]$script:createdCompanyId }
    }

    $script:createdKbId = Test-PostEndpoint "$ApiHost/api/2.1/kbs/" $kbBody "POST /kbs/" "/kbs/(\d+)/"

    if ($script:createdKbId) {
        Test-GetEndpoint "$ApiHost/api/2.1/kbs/$($script:createdKbId)/" "GET /kbs/$($script:createdKbId)/"

        $patchBody = @{ article = "Updated KB content" }
        Test-PatchEndpoint "$ApiHost/api/2.1/kbs/$($script:createdKbId)/" $patchBody "PATCH /kbs/$($script:createdKbId)/"
    }
} else {
    Write-Skip "POST /kbs/" "No KB category or subcategory found"
}

Test-GetEndpoint "$ApiHost/api/2.1/kbs/?limit=5" "GET /kbs/?limit=5"

# ============================================================
# SECTION 9: CONFIGURATIONS
# ============================================================
Write-Host "`n=== CONFIGURATIONS ===" -ForegroundColor Cyan

if ($script:createdCompanyId -and $script:configurationTypeId) {
    $configBody = @{
        name = "API-Test Config"
        company = @{ id = [int]$script:createdCompanyId }
        type = @{ id = $script:configurationTypeId }
    }

    $script:createdConfigurationId = Test-PostEndpoint "$ApiHost/api/2.1/configurations/" $configBody "POST /configurations/" "/configurations/(\d+)/"

    if ($script:createdConfigurationId) {
        Test-GetEndpoint "$ApiHost/api/2.1/configurations/$($script:createdConfigurationId)/" "GET /configurations/$($script:createdConfigurationId)/"

        $patchBody = @{ description = "Updated config" }
        Test-PatchEndpoint "$ApiHost/api/2.1/configurations/$($script:createdConfigurationId)/" $patchBody "PATCH /configurations/$($script:createdConfigurationId)/"
    }
} else {
    Write-Skip "POST /configurations/" "No test company created"
}

Test-GetEndpoint "$ApiHost/api/2.1/configurations/?limit=5" "GET /configurations/?limit=5"

# ITP3-11604: Configuration credentials GET
if ($script:createdConfigurationId) {
    Test-GetEndpoint "$ApiHost/api/2.1/configurations/$($script:createdConfigurationId)/credentials" "GET /configurations/$($script:createdConfigurationId)/credentials" -UseHeaders $CredentialHeaders
} else {
    Write-Skip "GET /configurations/{id}/credentials" "No test configuration created"
}

# ============================================================
# SECTION 10: FACILITIES
# ============================================================
Write-Host "`n=== FACILITIES ===" -ForegroundColor Cyan

if ($script:createdCompanyId -and $script:createdSiteId) {
    $facilityBody = @{
        name = "API-Test Facility"
        company = @{ id = [int]$script:createdCompanyId }
        site = @{ id = [int]$script:createdSiteId }
        type = @{ id = $script:facilityTypeId }
    }

    $script:createdFacilityId = Test-PostEndpoint "$ApiHost/api/2.1/facilities/" $facilityBody "POST /facilities/" "/facilities/(\d+)/"

    if ($script:createdFacilityId) {
        Test-GetEndpoint "$ApiHost/api/2.1/facilities/$($script:createdFacilityId)/" "GET /facilities/$($script:createdFacilityId)/"

        $patchBody = @{ description = "Updated facility" }
        Test-PatchEndpoint "$ApiHost/api/2.1/facilities/$($script:createdFacilityId)/" $patchBody "PATCH /facilities/$($script:createdFacilityId)/"
    }
} else {
    Write-Skip "POST /facilities/" "No test company or site created"
}

Test-GetEndpoint "$ApiHost/api/2.1/facilities/?limit=5" "GET /facilities/?limit=5"

# ============================================================
# SECTION 11: AGREEMENTS
# ============================================================
Write-Host "`n=== AGREEMENTS ===" -ForegroundColor Cyan

if ($script:createdCompanyId) {
    $agreementBody = @{
        name = "API-Test Agreement"
        company = @{ id = [int]$script:createdCompanyId }
        type = @{ id = $script:agreementTypeId }
    }

    $script:createdAgreementId = Test-PostEndpoint "$ApiHost/api/2.1/agreements/" $agreementBody "POST /agreements/" "/agreements/(\d+)/"

    if ($script:createdAgreementId) {
        Test-GetEndpoint "$ApiHost/api/2.1/agreements/$($script:createdAgreementId)/" "GET /agreements/$($script:createdAgreementId)/"

        $patchBody = @{ description = "Updated agreement" }
        Test-PatchEndpoint "$ApiHost/api/2.1/agreements/$($script:createdAgreementId)/" $patchBody "PATCH /agreements/$($script:createdAgreementId)/"
    }
} else {
    Write-Skip "POST /agreements/" "No test company created"
}

Test-GetEndpoint "$ApiHost/api/2.1/agreements/?limit=5" "GET /agreements/?limit=5"

# ============================================================
# SECTION 12: CABINETS
# ============================================================
Write-Host "`n=== CABINETS ===" -ForegroundColor Cyan

if ($script:createdCompanyId -and $script:createdSiteId) {
    $cabinetBody = @{
        name = "API-Test Cabinet"
        company = @{ id = [int]$script:createdCompanyId }
        site = @{ id = [int]$script:createdSiteId }
    }

    $script:createdCabinetId = Test-PostEndpoint "$ApiHost/api/2.1/cabinets/" $cabinetBody "POST /cabinets/" "/cabinets/(\d+)/"

    if ($script:createdCabinetId) {
        Test-GetEndpoint "$ApiHost/api/2.1/cabinets/$($script:createdCabinetId)/" "GET /cabinets/$($script:createdCabinetId)/"

        $patchBody = @{ description = "Updated cabinet" }
        Test-PatchEndpoint "$ApiHost/api/2.1/cabinets/$($script:createdCabinetId)/" $patchBody "PATCH /cabinets/$($script:createdCabinetId)/"
    }
} else {
    Write-Skip "POST /cabinets/" "No test company or site created"
}

Test-GetEndpoint "$ApiHost/api/2.1/cabinets/?limit=5" "GET /cabinets/?limit=5"

# ============================================================
# SECTION 13: IP NETWORKS
# ============================================================
Write-Host "`n=== IP NETWORKS ===" -ForegroundColor Cyan

if ($script:createdCompanyId -and $script:createdSiteId) {
    $ipNetworkBody = @{
        name = "API-Test Network"
        networkAddress = "10.0.$(Get-Random -Minimum 1 -Maximum 254).0"
        subnetMask = "255.255.255.0"
        company = @{ id = [int]$script:createdCompanyId }
        site = @{ id = [int]$script:createdSiteId }
    }

    $script:createdIpNetworkId = Test-PostEndpoint "$ApiHost/api/2.1/ipnetworks/" $ipNetworkBody "POST /ipnetworks/" "/ipnetworks/(\d+)/"

    if ($script:createdIpNetworkId) {
        Test-GetEndpoint "$ApiHost/api/2.1/ipnetworks/$($script:createdIpNetworkId)/" "GET /ipnetworks/$($script:createdIpNetworkId)/"

        $patchBody = @{ description = "Updated IP network" }
        Test-PatchEndpoint "$ApiHost/api/2.1/ipnetworks/$($script:createdIpNetworkId)/" $patchBody "PATCH /ipnetworks/$($script:createdIpNetworkId)/"
    }
} else {
    Write-Skip "POST /ipnetworks/" "No test company or site created"
}

Test-GetEndpoint "$ApiHost/api/2.1/ipnetworks/?limit=5" "GET /ipnetworks/?limit=5"

# ============================================================
# SECTION 14: ADDRESSES
# ============================================================
Write-Host "`n=== ADDRESSES ===" -ForegroundColor Cyan

Test-GetEndpoint "$ApiHost/api/2.1/addresses/?limit=5" "GET /addresses/?limit=5"

# ITP3-11604: POST /addresses with company in body (top-level route)
if ($script:createdCompanyId) {
    $addressBodyTopLevel = @{
        company = @{ id = [int]$script:createdCompanyId }
        address1 = "API-Test Address (top-level)"
        city = "TestCity"
        zip = "00000"
    }
    $script:createdTopLevelAddressId = Test-PostEndpoint "$ApiHost/api/2.1/addresses/" $addressBodyTopLevel "POST /addresses/ (company in body)" "/addresses/(\d+)/"

    if ($script:createdTopLevelAddressId) {
        Test-GetEndpoint "$ApiHost/api/2.1/addresses/$($script:createdTopLevelAddressId)/" "GET /addresses/$($script:createdTopLevelAddressId)/ (top-level created)"
    }
} else {
    Write-Skip "POST /addresses/ (company in body)" "No test company created"
}

# ITP3-11604: POST /addresses WITHOUT company should return 400
try {
    $noCompanyBody = @{ address1 = "Should fail"; city = "Test" }
    $jsonBody = $noCompanyBody | ConvertTo-Json -Depth 5 -Compress
    $response = Invoke-WebRequest -Uri "$ApiHost/api/2.1/addresses/" -Method Post -Headers $Headers -Body $jsonBody -UseBasicParsing
    Write-TestResult "POST /addresses/ (no company -> 400)" $false "Expected 400, got $($response.StatusCode)"
} catch {
    if ($_.Exception.Response.StatusCode.value__ -eq 400) {
        Write-TestResult "POST /addresses/ (no company -> 400)" $true "Correctly returned 400"
    } else {
        Write-TestResult "POST /addresses/ (no company -> 400)" $false $_.Exception.Message
    }
}

# ============================================================
# SECTION 15: ADDITIONAL CREDENTIALS
# ============================================================
Write-Host "`n=== ADDITIONAL CREDENTIALS ===" -ForegroundColor Cyan

if ($script:createdDeviceId) {
    $credBody = @{
        type = "API-Test Credential"
        description = "API-Test Credential"
        username = "testuser"
        password = "testpass123"
        portalObject = @{
            itemType = "Device"
            id = [int]$script:createdDeviceId
        }
    }

    $script:createdAdditionalCredentialId = Test-PostEndpoint "$ApiHost/api/2.1/additionalCredentials/" $credBody "POST /additionalCredentials/" "/additionalCredentials/(\d+)/" $CredentialHeaders

    if ($script:createdAdditionalCredentialId) {
        Test-GetEndpoint "$ApiHost/api/2.1/additionalCredentials/$($script:createdAdditionalCredentialId)/" "GET /additionalCredentials/$($script:createdAdditionalCredentialId)/" -UseHeaders $CredentialHeaders

        $patchBody = @{ description = "Updated credential" }
        Test-PatchEndpoint "$ApiHost/api/2.1/additionalCredentials/$($script:createdAdditionalCredentialId)/" $patchBody "PATCH /additionalCredentials/$($script:createdAdditionalCredentialId)/" $PatchCredentialHeaders
    }
} else {
    Write-Skip "POST /additionalCredentials/" "No test device created"
}

Test-GetEndpoint "$ApiHost/api/2.1/additionalCredentials/?limit=5" "GET /additionalCredentials/?limit=5" -UseHeaders $CredentialHeaders

# ============================================================
# SECTION 16: SHEETS - Skipped (requires complex structure)
# ============================================================
# Sheets require complex nested structure and are not tested here

# ============================================================
# SECTION 17: INTERACTIONS
# ============================================================
Write-Host "`n=== INTERACTIONS ===" -ForegroundColor Cyan

# Note: Interactions can only be added to certain object types (Account, Agreement, ConfigObject, Contact, Device, Document, Facility, KB, Cabinet, Site, Subnet)
# Company/Client is NOT supported for interactions
if ($script:createdDeviceId) {
    $interactionBody = @{
        note = "API-Test interaction note"
    }

    $script:createdInteractionId = Test-PostEndpoint "$ApiHost/api/2.1/interactions/device/$($script:createdDeviceId)/" $interactionBody "POST /interactions/device/{id}/" "/(\d+)/"

    # GET interactions
    Test-GetEndpoint "$ApiHost/api/2.1/interactions/device/$($script:createdDeviceId)/?limit=5" "GET /interactions/device/{id}/"
} else {
    Write-Skip "POST /interactions/" "No test device created"
}

# ============================================================
# SECTION 18: TEMPLATES
# ============================================================
Write-Host "`n=== TEMPLATES ===" -ForegroundColor Cyan

Test-GetEndpoint "$ApiHost/api/2.1/templates/?limit=5" "GET /templates/?limit=5"

if ($script:createdCompanyId) {
    Test-GetEndpoint "$ApiHost/api/2.1/templates/company/$($script:createdCompanyId)/" "GET /templates/company/{id}/"
}

# ============================================================
# SECTION 19: SYSTEM ENDPOINTS
# ============================================================
Write-Host "`n=== SYSTEM ===" -ForegroundColor Cyan

Test-GetEndpoint "$ApiHost/api/2.1/system/users/?limit=5" "GET /system/users/"
Test-GetEndpoint "$ApiHost/api/2.1/system/countries/?limit=5" "GET /system/countries/"
Test-GetEndpoint "$ApiHost/api/2.1/system/groups/securityGroups/?limit=5" "GET /system/groups/securityGroups/"
Test-GetEndpoint "$ApiHost/api/2.1/system/companies/mainContacts/?limit=5" "GET /system/companies/mainContacts/"

# ============================================================
# SECTION 20: LOGS
# ============================================================
Write-Host "`n=== LOGS ===" -ForegroundColor Cyan

# Logs may require date range filters
$startDate = (Get-Date).AddDays(-7).ToString("yyyy-MM-dd")
$endDate = (Get-Date).ToString("yyyy-MM-dd")

Test-GetEndpoint "$ApiHost/api/2.1/logs/userAccess/?startDate=$startDate&endDate=$endDate&limit=5" "GET /logs/userAccess/"
Test-GetEndpoint "$ApiHost/api/2.1/logs/adminAccess/?startDate=$startDate&endDate=$endDate&limit=5" "GET /logs/adminAccess/"
Test-GetEndpoint "$ApiHost/api/2.1/logs/loginLogout/?startDate=$startDate&endDate=$endDate&limit=5" "GET /logs/loginLogout/"

# ============================================================
# SECTION 21: FORMS
# ============================================================
Write-Host "`n=== FORMS ===" -ForegroundColor Cyan

Test-GetEndpoint "$ApiHost/api/2.1/forms/?limit=5" "GET /forms/"
Test-GetEndpoint "$ApiHost/api/2.1/forminstances/?limit=5" "GET /forminstances/"

# ============================================================
# SECTION 22: TYPES & CATEGORIES
# ============================================================
Write-Host "`n=== TYPES & CATEGORIES ===" -ForegroundColor Cyan

Test-GetEndpoint "$ApiHost/api/2.1/types/company/" "GET /types/company/"
Test-GetEndpoint "$ApiHost/api/2.1/types/account/" "GET /types/account/"
Test-GetEndpoint "$ApiHost/api/2.1/types/contact/" "GET /types/contact/"
Test-GetEndpoint "$ApiHost/api/2.1/types/device/" "GET /types/device/"
Test-GetEndpoint "$ApiHost/api/2.1/types/document/" "GET /types/document/"
Test-GetEndpoint "$ApiHost/api/2.1/types/facility/" "GET /types/facility/"
Test-GetEndpoint "$ApiHost/api/2.1/types/agreement/" "GET /types/agreement/"
Test-GetEndpoint "$ApiHost/api/2.1/types/configuration/" "GET /types/configuration/"
Test-GetEndpoint "$ApiHost/api/2.1/categories/kb/" "GET /categories/kb/"

# ============================================================
# SECTION 22b: TYPE MANAGEMENT (POST/PATCH/DELETE per kind)
# ============================================================
Write-Host "`n=== TYPE MANAGEMENT ===" -ForegroundColor Cyan

function Test-ExpectStatus {
    param($Method, $Url, $Body, $ExpectedStatus, $TestName, $ExpectedErrorCode = $null)

    $reqHeaders = $Headers
    if ($Method -eq "Patch") {
        $reqHeaders = @{
            "Content-Type" = "application/merge-patch+json"
            "Authorization" = "Basic $BasicAuth"
        }
    }

    $jsonBody = $null
    if ($Body) { $jsonBody = $Body | ConvertTo-Json -Depth 5 -Compress }

    try {
        $params = @{ Uri = $Url; Method = $Method; Headers = $reqHeaders; UseBasicParsing = $true }
        if ($jsonBody) { $params.Body = $jsonBody }
        $response = Invoke-WebRequest @params
        $actual = [int]$response.StatusCode
        if ($actual -eq $ExpectedStatus) {
            Write-TestResult $TestName $true "Status: $actual"
            return $response
        }
        Write-TestResult $TestName $false "Expected $ExpectedStatus, got $actual"
    } catch {
        $actual = 0
        if ($_.Exception.Response) { $actual = [int]$_.Exception.Response.StatusCode.value__ }
        if ($actual -eq $ExpectedStatus) {
            $okDetail = "Status: $actual"
            if ($ExpectedErrorCode) {
                try {
                    $stream = $_.Exception.Response.GetResponseStream()
                    $reader = New-Object System.IO.StreamReader($stream)
                    $body = $reader.ReadToEnd()
                    $reader.Close()
                    if ($body -match [regex]::Escape($ExpectedErrorCode)) {
                        $okDetail = "Status: $actual, errorCode: $ExpectedErrorCode"
                    } else {
                        Write-TestResult $TestName $false "Got $actual but missing errorCode '$ExpectedErrorCode' in body"
                        return $null
                    }
                } catch {}
            }
            Write-TestResult $TestName $true $okDetail
        } else {
            Write-TestResult $TestName $false "Expected $ExpectedStatus, got $actual"
        }
    }
    return $null
}

# Full cycle test per kind: create, rename, delete-unused (204), then duplicate (409)
$typeKinds = @("account","agreement","company","contact","device","document","facility","configuration")

foreach ($kind in $typeKinds) {
    $baseName = "APITestType-$kind-$(Get-Random -Maximum 999999)"

    # POST -> 201
    $createdTypeId = Test-PostEndpoint "$ApiHost/api/2.1/types/$kind/" @{ name = $baseName } "POST /types/$kind/" "/types/$kind/(\d+)/"

    if ($createdTypeId) {
        # Verify GET lists it
        $getResp = Invoke-RestMethod -Uri "$ApiHost/api/2.1/types/$kind/?name=$baseName" -Method Get -Headers $Headers
        if ($getResp.data.results.Count -gt 0) {
            Write-TestResult "GET /types/$kind/?name=$baseName lists created" $true "Found id $($getResp.data.results[0].id)"
        } else {
            Write-TestResult "GET /types/$kind/?name=$baseName lists created" $false "Not found"
        }

        # Duplicate POST -> 409 duplicateName
        Test-ExpectStatus "Post" "$ApiHost/api/2.1/types/$kind/" @{ name = $baseName } 409 "POST /types/$kind/ duplicate -> 409 duplicateName" "duplicateName"

        # POST empty name -> 400 requiredField
        Test-ExpectStatus "Post" "$ApiHost/api/2.1/types/$kind/" @{ name = "" } 400 "POST /types/$kind/ empty name -> 400 requiredField" "requiredField"

        # Rename PATCH -> 204
        $renamed = "$baseName-renamed"
        Test-PatchEndpoint "$ApiHost/api/2.1/types/$kind/$createdTypeId/" @{ name = $renamed } "PATCH /types/$kind/$createdTypeId/ rename"

        # PATCH not-found -> 404
        Test-ExpectStatus "Patch" "$ApiHost/api/2.1/types/$kind/999999999/" @{ name = "x" } 404 "PATCH /types/$kind/999999999/ -> 404"

        # DELETE unused -> 204
        Test-DeleteEndpoint "$ApiHost/api/2.1/types/$kind/$createdTypeId/" "DELETE /types/$kind/$createdTypeId/ (unused)"

        # DELETE not-found -> 404
        Test-ExpectStatus "Delete" "$ApiHost/api/2.1/types/$kind/$createdTypeId/" $null 404 "DELETE /types/$kind/$createdTypeId/ (already gone) -> 404"
    } else {
        Write-Skip "Type $kind rename/delete cycle" "POST failed"
    }
}

# typeInUse 409: create a device type, attach a device, try delete -> 409 typeInUse
if ($script:createdCompanyId) {
    $inUseName = "APITestType-device-inuse-$(Get-Random -Maximum 999999)"
    $inUseTypeId = Test-PostEndpoint "$ApiHost/api/2.1/types/device/" @{ name = $inUseName } "POST /types/device/ (for inuse test)" "/types/device/(\d+)/"

    if ($inUseTypeId) {
        $deviceBody = @{
            name = "APITestDevice-inuse-$(Get-Random -Maximum 999999)"
            hostName = "apitest-inuse-host"
            company = @{ id = $script:createdCompanyId }
            type = @{ id = [int]$inUseTypeId }
            site = @{ id = $script:createdSiteId }
        }
        $inUseDeviceId = Test-PostEndpoint "$ApiHost/api/2.1/devices/" $deviceBody "POST /devices/ using new type" "/devices/(\d+)/"

        if ($inUseDeviceId) {
            Test-ExpectStatus "Delete" "$ApiHost/api/2.1/types/device/$inUseTypeId/" $null 409 "DELETE /types/device/$inUseTypeId/ while in use -> 409 typeInUse" "typeInUse"

            # Cleanup: delete device first, then type
            Test-DeleteEndpoint "$ApiHost/api/2.1/devices/$inUseDeviceId/" "DELETE /devices/$inUseDeviceId/ (inuse cleanup)"
            Test-DeleteEndpoint "$ApiHost/api/2.1/types/device/$inUseTypeId/" "DELETE /types/device/$inUseTypeId/ (now unused)"
        } else {
            Test-DeleteEndpoint "$ApiHost/api/2.1/types/device/$inUseTypeId/" "DELETE /types/device/$inUseTypeId/ (cleanup)"
        }
    }
} else {
    Write-Skip "typeInUse 409 test" "No test company/site available"
}

# ============================================================
# SECTION 22c: KB CATEGORY & SUBCATEGORY MANAGEMENT
# ============================================================
Write-Host "`n=== KB CATEGORY MANAGEMENT ===" -ForegroundColor Cyan

$kbCatName = "APITest-KBCat-$(Get-Random -Maximum 999999)"
$createdKbCatId = Test-PostEndpoint "$ApiHost/api/2.1/categories/kb/" @{ name = $kbCatName } "POST /categories/kb/" "/categories/kb/(\d+)/"

if ($createdKbCatId) {
    # Verify GET
    $kbGet = Invoke-RestMethod -Uri "$ApiHost/api/2.1/categories/kb/?name=$kbCatName" -Method Get -Headers $Headers
    if ($kbGet.data.results.Count -gt 0) {
        Write-TestResult "GET /categories/kb/?name=$kbCatName lists created" $true "Found"
    } else {
        Write-TestResult "GET /categories/kb/?name=$kbCatName lists created" $false "Not found"
    }

    # Duplicate -> 409 duplicateName
    Test-ExpectStatus "Post" "$ApiHost/api/2.1/categories/kb/" @{ name = $kbCatName } 409 "POST /categories/kb/ duplicate -> 409 duplicateName" "duplicateName"

    # Required field -> 400
    Test-ExpectStatus "Post" "$ApiHost/api/2.1/categories/kb/" @{ name = "" } 400 "POST /categories/kb/ empty name -> 400 requiredField" "requiredField"

    # Rename -> 204
    $kbCatRenamed = "$kbCatName-renamed"
    Test-PatchEndpoint "$ApiHost/api/2.1/categories/kb/$createdKbCatId/" @{ name = $kbCatRenamed } "PATCH /categories/kb/$createdKbCatId/ rename"

    # --- Subcategory cycle ---
    $subName = "APITest-Sub-$(Get-Random -Maximum 999999)"
    $createdSubId = Test-PostEndpoint "$ApiHost/api/2.1/categories/kb/$createdKbCatId/subcategories/" @{ name = $subName } "POST /categories/kb/$createdKbCatId/subcategories/" "/subcategories/(\d+)/"

    if ($createdSubId) {
        # Duplicate subcategory within same parent -> 409
        Test-ExpectStatus "Post" "$ApiHost/api/2.1/categories/kb/$createdKbCatId/subcategories/" @{ name = $subName } 409 "POST subcategory duplicate -> 409 duplicateName" "duplicateName"

        # Rename subcategory
        Test-PatchEndpoint "$ApiHost/api/2.1/categories/kb/$createdKbCatId/subcategories/$createdSubId/" @{ name = "$subName-renamed" } "PATCH subcategory rename"

        # Delete unused subcategory -> 204
        Test-DeleteEndpoint "$ApiHost/api/2.1/categories/kb/$createdKbCatId/subcategories/$createdSubId/" "DELETE empty subcategory -> 204"
    }

    # Delete unused category -> 204 (also cascade-deletes any empty subs)
    Test-DeleteEndpoint "$ApiHost/api/2.1/categories/kb/$createdKbCatId/" "DELETE empty KB category -> 204"
}

# categoryInUse 409 test: create category, create KB article in it, try delete -> 409
if ($script:createdCompanyId) {
    $inUseCatName = "APITest-KBCat-inuse-$(Get-Random -Maximum 999999)"
    $inUseCatId = Test-PostEndpoint "$ApiHost/api/2.1/categories/kb/" @{ name = $inUseCatName } "POST /categories/kb/ (for inuse test)" "/categories/kb/(\d+)/"

    if ($inUseCatId) {
        $kbBody = @{
            name = "APITest-KB-inuse-$(Get-Random -Maximum 999999)"
            company = @{ id = $script:createdCompanyId }
            category = @{ id = [int]$inUseCatId }
            description = "Created for categoryInUse test"
        }
        $inUseKbId = Test-PostEndpoint "$ApiHost/api/2.1/kbs/" $kbBody "POST /kbs/ in new category" "/kbs/(\d+)/"

        if ($inUseKbId) {
            Test-ExpectStatus "Delete" "$ApiHost/api/2.1/categories/kb/$inUseCatId/" $null 409 "DELETE KB category in use -> 409 categoryInUse" "categoryInUse"

            Test-DeleteEndpoint "$ApiHost/api/2.1/kbs/$inUseKbId/" "DELETE /kbs/$inUseKbId/ (cleanup)"
            Test-DeleteEndpoint "$ApiHost/api/2.1/categories/kb/$inUseCatId/" "DELETE KB category (now unused)"
        } else {
            Test-DeleteEndpoint "$ApiHost/api/2.1/categories/kb/$inUseCatId/" "DELETE KB category (cleanup, no article created)"
        }
    }
} else {
    Write-Skip "categoryInUse 409 test" "No test company available"
}

# ============================================================
# SECTION 22d: FOLDERS & FOLDER FILES
# Exercises the per-object folder tree + multipart-file endpoints
# against the test document created in SECTION 7.
# ============================================================
Write-Host "`n=== FOLDERS & FOLDER FILES ===" -ForegroundColor Cyan

if ($script:createdDocumentId) {
    $docBase = "$ApiHost/api/2.1/documents/$($script:createdDocumentId)"

    # 1. First GET auto-creates Root_Folder.
    $rootResp = Test-GetEndpoint "$docBase/folders/" "GET /documents/{id}/folders/ (auto-creates Root_Folder)"
    $script:createdRootFolderId = $null
    if ($rootResp -and $rootResp.data.results) {
        $rootMatch = $rootResp.data.results | Where-Object { $_.name -eq "Root_Folder" } | Select-Object -First 1
        if ($rootMatch) { $script:createdRootFolderId = $rootMatch.id }
    }

    if ($script:createdRootFolderId) {
        # 2. Create a child folder under Root_Folder.
        $folderBody = @{ name = "API-Test Folder"; description = "Created by test script"; parentFolderId = $script:createdRootFolderId }
        $script:createdFolderId = Test-PostEndpoint "$docBase/folders/" $folderBody "POST /documents/{id}/folders/" "/folders/(\d+)/"

        # 3. Duplicate-name POST should return 409 duplicateName.
        try {
            $dup = @{ name = "API-Test Folder"; parentFolderId = $script:createdRootFolderId } | ConvertTo-Json -Depth 5 -Compress
            $r = Invoke-WebRequest -Uri "$docBase/folders/" -Method Post -Headers $Headers -Body $dup -UseBasicParsing
            Write-TestResult "POST duplicate folder name -> 409" $false "Got status $($r.StatusCode)"
        } catch {
            if ($_.Exception.Response.StatusCode.value__ -eq 409) {
                Write-TestResult "POST duplicate folder name -> 409" $true "Correctly rejected as duplicate"
            } else {
                Write-TestResult "POST duplicate folder name -> 409" $false $_.Exception.Message
            }
        }

        # 4. PATCH rename the folder.
        if ($script:createdFolderId) {
            $patchBody = @{ name = "API-Test Folder Renamed"; description = "Updated" }
            Test-PatchEndpoint "$docBase/folders/$($script:createdFolderId)/" $patchBody "PATCH /documents/{id}/folders/{fid}/"
            Test-GetEndpoint "$docBase/folders/$($script:createdFolderId)/" "GET /documents/{id}/folders/$($script:createdFolderId)/"
        }

        # 5. PATCH Root_Folder should be rejected.
        try {
            $rootPatch = @{ name = "not-allowed" } | ConvertTo-Json -Depth 5 -Compress
            $r = Invoke-WebRequest -Uri "$docBase/folders/$($script:createdRootFolderId)/" -Method Patch -Headers $PatchCredentialHeaders -Body $rootPatch -UseBasicParsing
            Write-TestResult "PATCH Root_Folder -> rejected" $false "Got status $($r.StatusCode)"
        } catch {
            if ($_.Exception.Response.StatusCode.value__ -eq 400) {
                Write-TestResult "PATCH Root_Folder -> rejected" $true "Correctly rejected (400)"
            } else {
                Write-TestResult "PATCH Root_Folder -> rejected" $false $_.Exception.Message
            }
        }

        # 6. Upload a file (multipart) to the created folder.
        $script:createdFolderFileId = $null
        if ($script:createdFolderId) {
            $tmpFile = Join-Path $env:TEMP "api-test-folder-file.txt"
            "Folder file contents created by the API test script at $(Get-Date -Format o)" | Set-Content -Path $tmpFile -Encoding UTF8

            try {
                $uploadResp = Invoke-WebRequest -Uri "$docBase/folderFiles/$($script:createdFolderId)/" `
                    -Method Post `
                    -Headers @{ "Authorization" = $Headers["Authorization"] } `
                    -Form @{ file = Get-Item $tmpFile; description = "API smoke test" } `
                    -UseBasicParsing
                if ($uploadResp.StatusCode -eq 201) {
                    $loc = $uploadResp.Headers["Location"]
                    if ($loc -match "/folderFiles/\d+/(\d+)/") { $script:createdFolderFileId = $matches[1] }
                    Write-TestResult "POST /documents/{id}/folderFiles/{fid}/ (multipart)" $true "Created file $script:createdFolderFileId"
                } else {
                    Write-TestResult "POST /documents/{id}/folderFiles/{fid}/ (multipart)" $false "Status $($uploadResp.StatusCode)"
                }
            } catch {
                Write-TestResult "POST /documents/{id}/folderFiles/{fid}/ (multipart)" $false $_.Exception.Message
            }

            Remove-Item $tmpFile -ErrorAction SilentlyContinue
        }

        # 7. List + GET download.
        if ($script:createdFolderFileId) {
            Test-GetEndpoint "$docBase/folderFiles/$($script:createdFolderId)/" "GET /documents/{id}/folderFiles/{fid}/ (list)"

            try {
                $dl = Invoke-WebRequest -Uri "$docBase/folderFiles/$($script:createdFolderId)/$($script:createdFolderFileId)/" -Headers $Headers -UseBasicParsing
                if ($dl.StatusCode -eq 200 -and $dl.Content -and ($dl.Content.Length -gt 0 -or $dl.RawContentLength -gt 0)) {
                    Write-TestResult "GET /folderFiles/{fid}/{fileId}/ (binary download)" $true "Received $($dl.RawContentLength) bytes"
                } else {
                    Write-TestResult "GET /folderFiles/{fid}/{fileId}/ (binary download)" $false "Empty response"
                }
            } catch {
                Write-TestResult "GET /folderFiles/{fid}/{fileId}/ (binary download)" $false $_.Exception.Message
            }

            # 8. PATCH rename the file.
            $filePatch = @{ fileName = "renamed-by-test.txt"; description = "renamed" }
            Test-PatchEndpoint "$docBase/folderFiles/$($script:createdFolderId)/$($script:createdFolderFileId)/" $filePatch "PATCH folderFile rename"

            # 9. DELETE folder with file inside should return 409 folderInUse.
            try {
                $r = Invoke-WebRequest -Uri "$docBase/folders/$($script:createdFolderId)/" -Method Delete -Headers $Headers -UseBasicParsing
                Write-TestResult "DELETE non-empty folder -> 409 folderInUse" $false "Got status $($r.StatusCode)"
            } catch {
                if ($_.Exception.Response.StatusCode.value__ -eq 409) {
                    Write-TestResult "DELETE non-empty folder -> 409 folderInUse" $true "Correctly rejected"
                } else {
                    Write-TestResult "DELETE non-empty folder -> 409 folderInUse" $false $_.Exception.Message
                }
            }

            # 10. Cleanup: delete the file, then the folder.
            Test-DeleteEndpoint "$docBase/folderFiles/$($script:createdFolderId)/$($script:createdFolderFileId)/" "DELETE folderFile"
        }

        if ($script:createdFolderId) {
            Test-DeleteEndpoint "$docBase/folders/$($script:createdFolderId)/" "DELETE /documents/{id}/folders/$($script:createdFolderId)/"
        }
    } else {
        Write-Skip "Folders & folderFiles section" "Could not resolve Root_Folder id"
    }
} else {
    Write-Skip "Folders & folderFiles section" "No test document created"
}

# ============================================================
# SECTION 22e: RELATIONSHIPS
# Exercises the per-object relationships (invLinks) endpoints
# using the test device + document created earlier.
# ============================================================
Write-Host "`n=== RELATIONSHIPS ===" -ForegroundColor Cyan

if ($script:createdDeviceId -and $script:createdDocumentId) {
    $devBase = "$ApiHost/api/2.1/devices/$($script:createdDeviceId)"
    $docBase = "$ApiHost/api/2.1/documents/$($script:createdDocumentId)"

    # 1. GET existing relationships (may be empty).
    Test-GetEndpoint "$devBase/relationships/" "GET /devices/{id}/relationships/"

    # 2. POST a link device -> document.
    $body = @{ target = @{ itemType = "Document"; id = [int]$script:createdDocumentId }; notes = "API test link" }
    $script:createdRelationshipId = Test-PostEndpoint "$devBase/relationships/" $body "POST /devices/{id}/relationships/ (Device <-> Document)" "/relationships/(\d+)/"

    # 3. Symmetric GET from the document side should list the same linkId.
    if ($script:createdRelationshipId) {
        try {
            $docRels = Invoke-RestMethod -Uri "$docBase/relationships/" -Method Get -Headers $Headers
            $match = $docRels.data.results | Where-Object { $_.id -eq [int]$script:createdRelationshipId } | Select-Object -First 1
            if ($match -and $match.target.itemType -eq "Device" -and [int]$match.target.id -eq [int]$script:createdDeviceId) {
                Write-TestResult "Relationship visible from both sides" $true "doc /rels/ lists linkId $script:createdRelationshipId"
            } else {
                Write-TestResult "Relationship visible from both sides" $false "linkId not found on the document side"
            }
        } catch {
            Write-TestResult "Relationship visible from both sides" $false $_.Exception.Message
        }
    }

    # 4. Duplicate POST should return 409 (same direction).
    try {
        $dupBody = @{ target = @{ itemType = "Document"; id = [int]$script:createdDocumentId } } | ConvertTo-Json -Depth 5 -Compress
        $r = Invoke-WebRequest -Uri "$devBase/relationships/" -Method Post -Headers $Headers -Body $dupBody -UseBasicParsing
        Write-TestResult "POST duplicate relationship -> 409" $false "Got status $($r.StatusCode)"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 409) {
            Write-TestResult "POST duplicate relationship -> 409" $true "Correctly rejected"
        } else {
            Write-TestResult "POST duplicate relationship -> 409" $false $_.Exception.Message
        }
    }

    # 5. Duplicate POST in reverse direction should also return 409.
    try {
        $revBody = @{ target = @{ itemType = "Device"; id = [int]$script:createdDeviceId } } | ConvertTo-Json -Depth 5 -Compress
        $r = Invoke-WebRequest -Uri "$docBase/relationships/" -Method Post -Headers $Headers -Body $revBody -UseBasicParsing
        Write-TestResult "POST reverse-direction duplicate -> 409" $false "Got status $($r.StatusCode)"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 409) {
            Write-TestResult "POST reverse-direction duplicate -> 409" $true "Correctly rejected"
        } else {
            Write-TestResult "POST reverse-direction duplicate -> 409" $false $_.Exception.Message
        }
    }

    # 6. Self-link should return 400.
    try {
        $selfBody = @{ target = @{ itemType = "Device"; id = [int]$script:createdDeviceId } } | ConvertTo-Json -Depth 5 -Compress
        $r = Invoke-WebRequest -Uri "$devBase/relationships/" -Method Post -Headers $Headers -Body $selfBody -UseBasicParsing
        Write-TestResult "POST self-link -> 400" $false "Got status $($r.StatusCode)"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 400) {
            Write-TestResult "POST self-link -> 400" $true "Correctly rejected"
        } else {
            Write-TestResult "POST self-link -> 400" $false $_.Exception.Message
        }
    }

    # 7. Non-existent target id -> 400.
    try {
        $badBody = @{ target = @{ itemType = "Document"; id = 99999999 } } | ConvertTo-Json -Depth 5 -Compress
        $r = Invoke-WebRequest -Uri "$devBase/relationships/" -Method Post -Headers $Headers -Body $badBody -UseBasicParsing
        Write-TestResult "POST non-existent target -> 400" $false "Got status $($r.StatusCode)"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 400) {
            Write-TestResult "POST non-existent target -> 400" $true "Correctly rejected"
        } else {
            Write-TestResult "POST non-existent target -> 400" $false $_.Exception.Message
        }
    }

    # 8. PATCH notes and re-GET.
    if ($script:createdRelationshipId) {
        $patchBody = @{ notes = "updated by API test" }
        Test-PatchEndpoint "$devBase/relationships/$($script:createdRelationshipId)/" $patchBody "PATCH /devices/{id}/relationships/{lid}/"
        Test-GetEndpoint "$devBase/relationships/$($script:createdRelationshipId)/" "GET single relationship after patch"
    }

    # 9. Cleanup: delete the relationship.
    if ($script:createdRelationshipId) {
        Test-DeleteEndpoint "$devBase/relationships/$($script:createdRelationshipId)/" "DELETE /devices/{id}/relationships/$($script:createdRelationshipId)/"
    }
} else {
    Write-Skip "Relationships section" "Needs both a test device and a test document"
}

# ============================================================
# SECTION 23: PAGINATION (Cursor only - offset is deprecated)
# ============================================================
Write-Host "`n=== PAGINATION ===" -ForegroundColor Cyan

# Test cursor pagination (recommended) - default sort is now by id
$cursorResponse = Test-GetEndpoint "$ApiHost/api/2.1/companies/?limit=2" "GET /companies/?limit=2 (with cursor)"
if ($cursorResponse -and $cursorResponse.data.nextCursor) {
    $nextCursor = $cursorResponse.data.nextCursor
    Write-Host "       nextCursor: $nextCursor" -ForegroundColor Gray
    Test-GetEndpoint "$ApiHost/api/2.1/companies/?limit=2&cursor=$nextCursor" "GET /companies/?cursor=$nextCursor"
}

# Test cursor on devices - default sort is now by id
$devicesCursor = Test-GetEndpoint "$ApiHost/api/2.1/devices/?limit=2" "GET /devices/?limit=2 (with cursor)"
if ($devicesCursor -and $devicesCursor.data.nextCursor) {
    Write-Host "       nextCursor: $($devicesCursor.data.nextCursor)" -ForegroundColor Gray
}

# ============================================================
# ITP3-11604: PUT REJECTION TEST
# ============================================================
Write-Host "`n=== PUT REJECTION ===" -ForegroundColor Cyan

try {
    $putBody = @{ description = "PUT should be rejected" } | ConvertTo-Json -Depth 5 -Compress
    $response = Invoke-WebRequest -Uri "$ApiHost/api/2.1/devices/1/" -Method Put -Headers $Headers -Body $putBody -UseBasicParsing
    $content = $response.Content
    if ($content -match "405" -and $content -match "PATCH") {
        Write-TestResult "PUT /devices/1/ -> 405 with PATCH guidance" $true "Correctly rejected"
    } else {
        Write-TestResult "PUT /devices/1/ -> 405 with PATCH guidance" $false "Got status $($response.StatusCode) without 405 in body"
    }
} catch {
    if ($_.Exception.Response.StatusCode.value__ -eq 405) {
        Write-TestResult "PUT /devices/1/ -> 405 with PATCH guidance" $true "Correctly returned 405"
    } else {
        # The substop pattern returns 200 with 405 in body - check the response body
        Write-TestResult "PUT /devices/1/ -> 405 with PATCH guidance" $false $_.Exception.Message
    }
}

# ============================================================
# ITP3-11604: V4 URL FORMAT TEST
# ============================================================
Write-Host "`n=== V4 URL FORMAT ===" -ForegroundColor Cyan

if ($script:createdDeviceId) {
    $devResponse = Invoke-RestMethod -Uri "$ApiHost/api/2.1/devices/$($script:createdDeviceId)/" -Method Get -Headers $Headers
    $url = $devResponse.data.results[0].url
    if ($url -match "/v4/app/devices/") {
        Write-TestResult "Device URL v4 format" $true $url
    } else {
        Write-TestResult "Device URL v4 format" $false "Expected /v4/app/devices/..., got: $url"
    }
} else {
    Write-Skip "Device URL v4 format" "No test device created"
}

if ($script:createdCompanyId) {
    $compResponse = Invoke-RestMethod -Uri "$ApiHost/api/2.1/companies/$($script:createdCompanyId)/" -Method Get -Headers $Headers
    $url = $compResponse.data.results[0].url
    if ($url -match "/v4/app/companies/") {
        Write-TestResult "Company URL v4 format" $true $url
    } else {
        Write-TestResult "Company URL v4 format" $false "Expected /v4/app/companies/..., got: $url"
    }
} else {
    Write-Skip "Company URL v4 format" "No test company created"
}

# ============================================================
# TEST SUMMARY
# ============================================================
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "           TEST SUMMARY" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  PASSED: $($script:passCount)" -ForegroundColor Green
Write-Host "  FAILED: $($script:failCount)" -ForegroundColor Red
Write-Host "  SKIPPED: $($script:skipCount)" -ForegroundColor Yellow
Write-Host "  TOTAL:  $($script:passCount + $script:failCount + $script:skipCount)" -ForegroundColor White
Write-Host "========================================" -ForegroundColor Cyan

# ============================================================
# CLEANUP PROMPT
# ============================================================
Write-Host "`n"
$cleanup = Read-Host "Delete all test data created during this run? (Y/n)"

if ($cleanup -ne 'n' -and $cleanup -ne 'N') {
    Write-Host "`n=== CLEANUP ===" -ForegroundColor Magenta

    # Delete in reverse dependency order

    if ($script:createdInteractionId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/interactions/$($script:createdInteractionId)/" "DELETE /interactions/$($script:createdInteractionId)/"
    }

    # ITP3-11604: cleanup top-level address
    if ($script:createdTopLevelAddressId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/addresses/$($script:createdTopLevelAddressId)/" "DELETE /addresses/$($script:createdTopLevelAddressId)/ (top-level)"
    }

    if ($script:createdAdditionalCredentialId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/additionalCredentials/$($script:createdAdditionalCredentialId)/" "DELETE /additionalCredentials/$($script:createdAdditionalCredentialId)/"
    }

    if ($script:createdIpNetworkId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/ipnetworks/$($script:createdIpNetworkId)/" "DELETE /ipnetworks/$($script:createdIpNetworkId)/"
    }

    if ($script:createdCabinetId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/cabinets/$($script:createdCabinetId)/" "DELETE /cabinets/$($script:createdCabinetId)/"
    }

    if ($script:createdAgreementId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/agreements/$($script:createdAgreementId)/" "DELETE /agreements/$($script:createdAgreementId)/"
    }

    if ($script:createdFacilityId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/facilities/$($script:createdFacilityId)/" "DELETE /facilities/$($script:createdFacilityId)/"
    }

    if ($script:createdConfigurationId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/configurations/$($script:createdConfigurationId)/" "DELETE /configurations/$($script:createdConfigurationId)/"
    }

    if ($script:createdKbId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/kbs/$($script:createdKbId)/" "DELETE /kbs/$($script:createdKbId)/"
    }

    if ($script:createdDocumentId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/documents/$($script:createdDocumentId)/" "DELETE /documents/$($script:createdDocumentId)/"
    }

    # Device sub-entities first, then device
    if ($script:createdDeviceNoteId -and $script:createdDeviceId) {
        # Notes don't have DELETE endpoint, skip
    }

    if ($script:createdDeviceMurlId -and $script:createdDeviceId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/devices/$($script:createdDeviceId)/managementUrls/$($script:createdDeviceMurlId)/" "DELETE /devices/{id}/managementUrls/$($script:createdDeviceMurlId)/"
    }

    if ($script:createdDeviceId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/devices/$($script:createdDeviceId)/" "DELETE /devices/$($script:createdDeviceId)/"
    }

    if ($script:createdSiteId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/sites/$($script:createdSiteId)/" "DELETE /sites/$($script:createdSiteId)/"
    }

    if ($script:createdContactId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/contacts/$($script:createdContactId)/" "DELETE /contacts/$($script:createdContactId)/"
    }

    if ($script:createdAccountId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/accounts/$($script:createdAccountId)/" "DELETE /accounts/$($script:createdAccountId)/"
    }

    # Company last (parent of all)
    if ($script:createdCompanyId) {
        Test-DeleteEndpoint "$ApiHost/api/2.1/companies/$($script:createdCompanyId)/" "DELETE /companies/$($script:createdCompanyId)/"
    }

    Write-Host "`nCleanup complete!" -ForegroundColor Green
} else {
    Write-Host "`nSkipping cleanup. Test data preserved." -ForegroundColor Yellow
    if ($script:createdCompanyId) {
        Write-Host "  Test Company ID: $($script:createdCompanyId)" -ForegroundColor Gray
    }
}

Write-Host "`nTests completed!" -ForegroundColor Green
