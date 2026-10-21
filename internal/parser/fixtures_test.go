package parser_test

var referenceProbeMatch = []byte(`<?xml version="1.0" encoding="UTF-8"?>
<ProbeMatch>
  <Uuid>urn:uuid:test-device-1</Uuid>
  <Types>inquiry</Types>
  <MAC>00:11:22:33:44:55</MAC>
  <IPv4Address>192.0.2.20</IPv4Address>
  <Activated>TRUE</Activated>
  <DeviceModel>DS-TEST-1</DeviceModel>
  <SerialNumber>SN-TEST-1</SerialNumber>
  <SoftwareVersion>V1.2.3</SoftwareVersion>
</ProbeMatch>`)

var observedProbeMatch = []byte(`<ProbeMatch>
  <Uuid>urn:uuid:observed-1</Uuid>
  <Types>inquiry</Types>
  <MAC>AA-BB-CC-DD-EE-FF</MAC>
  <IPv4Address>198.51.100.20</IPv4Address>
  <Activated>false</Activated>
  <DeviceDescription>Observed device</DeviceDescription>
  <DeviceSN>OBS-SN-1</DeviceSN>
  <Salt>sanitized</Salt>
</ProbeMatch>`)

var helloWithoutUUID = []byte(`<Hello>
  <Types>hello</Types>
  <MAC>AA-BB-CC-DD-EE-FF</MAC>
  <IPv4Address>198.51.100.20</IPv4Address>
  <Activated>false</Activated>
  <DeviceDescription>Observed device</DeviceDescription>
  <DeviceSN>OBS-SN-1</DeviceSN>
</Hello>`)
