package analyze

import (
	"github.com/lkarlslund/adalanche/modules/engine"
	"github.com/lkarlslund/adalanche/modules/windowssecurity"
)

var (
	LocalMachineSID         = engine.NewAttribute("localMachineSID")
	LocalMachineSIDOriginal = engine.NewAttribute("localMachineSIDOriginal")
	AbsolutePath            = engine.NewAttribute("absolutePath")
	ShareType               = engine.NewAttribute("shareType")
	ServiceStart            = engine.NewAttribute("serviceStart")
	ServiceType             = engine.NewAttribute("serviceType")

	EdgeLocalAdminRights = engine.NewEdge("AdminRights").Tag("Granted")
	EdgeLocalRDPRights   = engine.NewEdge("RDPRights").RegisterProbabilityCalculator(func(source, target *engine.Object) engine.Probability {
		var probability engine.Probability
		/* ENDLESS LOOPS
			target.Edges(engine.In).Range(func(potential *engine.Object, edge engine.EdgeBitmap) bool {
			sid := potential.SID()
			if sid.IsBlank() {
				return true // continue
			}
			if sid == windowssecurity.InteractiveSID || sid == windowssecurity.RemoteInteractiveSID || sid == windowssecurity.AuthenticatedUsersSID || sid == windowssecurity.EveryoneSID {
				probability = edge.MaxProbability(potential, target)
				return false // break
			}
			return true
		})*/
		if probability < 30 {
			probability = 30
		}
		return probability
	}).Tag("Granted").Tag("Pivot")
	EdgeLocalDCOMRights              = engine.NewEdge("DCOMRights").RegisterProbabilityCalculator(func(source, target *engine.Object) engine.Probability { return 50 }).Tag("Granted")
	EdgeLocalSMSAdmins               = engine.NewEdge("SMSAdmins").RegisterProbabilityCalculator(func(source, target *engine.Object) engine.Probability { return 50 }).Tag("Granted")
	EdgeLocalSessionLastDay          = engine.NewEdge("SessionLastDay").RegisterProbabilityCalculator(func(source, target *engine.Object) engine.Probability { return 80 }).Tag("Pivot")
	EdgeLocalSessionLastWeek         = engine.NewEdge("SessionLastWeek").RegisterProbabilityCalculator(func(source, target *engine.Object) engine.Probability { return 55 }).Tag("Pivot")
	EdgeLocalSessionLastMonth        = engine.NewEdge("SessionLastMonth").RegisterProbabilityCalculator(func(source, target *engine.Object) engine.Probability { return 30 }).Tag("Pivot")
	EdgeHasServiceAccountCredentials = engine.NewEdge("SvcAccntCreds").Tag("Pivot")
	EdgeHasAutoAdminLogonCredentials = engine.NewEdge("AutoAdminLogonCreds").Tag("Pivot")
	EdgeRunsExecutable               = engine.NewEdge("RunsExecutable")
	EdgeHosts                        = engine.NewEdge("Hosts")
	EdgeExecuted                     = engine.NewEdge("Executed")
	EdgeMemberOfGroup                = engine.NewEdge("MemberOfGroup")
	EdgeFileWrite                    = engine.NewEdge("FileWrite")
	EdgeFileRead                     = engine.NewEdge("FileRead")
	EdgeShares                       = engine.NewEdge("Shares").Describe("Machine offers a file share")
	EdgeRegistryOwns                 = engine.NewEdge("RegistryOwns")
	EdgeRegistryWrite                = engine.NewEdge("RegistryWrite")
	EdgeRegistryModifyDACL           = engine.NewEdge("RegistryModifyDACL")
	EdgeRegistryModifyOwner          = engine.NewEdge("RegistryModifyOwner")

	EdgeSeBackupPrivilege        = engine.NewEdge("SeBackupPrivilege")
	EdgeSeRestorePrivilege       = engine.NewEdge("SeRestorePrivilege")
	EdgeSeTakeOwnershipPrivilege = engine.NewEdge("SeTakeOwnershipPrivilege")

	EdgeSeAssignPrimaryToken   = engine.NewEdge("SeAssignPrimaryToken").Tag("Pivot")
	EdgeSeCreateToken          = engine.NewEdge("SeCreateToken").Tag("Pivot")
	EdgeSeDebug                = engine.NewEdge("SeDebug").Tag("Pivot")
	EdgeSeImpersonate          = engine.NewEdge("SeImpersonate").RegisterProbabilityCalculator(func(source, target *engine.Object) engine.Probability { return 20 }).Tag("Pivot")
	EdgeSeLoadDriver           = engine.NewEdge("SeLoadDriver").Tag("Pivot")
	EdgeSeManageVolume         = engine.NewEdge("SeManageVolume").Tag("Pivot")
	EdgeSeTakeOwnership        = engine.NewEdge("SeTakeOwnership").Tag("Pivot")
	EdgeSeTrustedCredManAccess = engine.NewEdge("SeTrustedCredManAccess").Tag("Pivot")
	EdgeSeTcb                  = engine.NewEdge("SeTcb").Tag("Pivot")

	EdgeSeNetworkLogonRight = engine.NewEdge("SeNetworkLogonRight").RegisterProbabilityCalculator(func(source, target *engine.Object) engine.Probability { return 10 })
	// RDPRight used ... EdgeSeRemoteInteractiveLogonRight = engine.NewEdge("SeRemoteInteractiveLogonRight").RegisterProbabilityCalculator(func(source, target *engine.Object) engine.Probability { return 10 })

	// SeDenyNetworkLogonRight
	// SeDenyInteractiveLogonRight
	// SeDenyRemoteInteractiveLogonRight

	EdgeSIDCollision = engine.NewEdge("SIDCollision").Tag("Informative").RegisterProbabilityCalculator(func(source, target *engine.Object) engine.Probability { return 0 })

	DNSHostname         = engine.NewAttribute("dnsHostName")
	EdgeControlsUpdates = engine.NewEdge("ControlsUpdates").Tag("Affects")
	WUServer            = engine.NewAttribute("wuServer")
	SCCMServer          = engine.NewAttribute("sccmServer")

	EdgePublishes = engine.NewEdge("Publishes").Tag("Informative")

	ObjectTypeShare = engine.NewObjectType("Share", "Share")
)

func MapSID(original, new, input windowssecurity.SID) windowssecurity.SID {
	// If input SID is one longer than machine sid
	if input.Components() == original.Components()+1 {
		// And it matches the original SID
		if input.StripRID() == original {
			// Return mapped SID
			return new.AddComponent(input.RID())
		}
	}
	// No mapping
	return input
}
