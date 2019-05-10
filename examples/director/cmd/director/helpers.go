package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"open-match.dev/open-match/internal/pb"
	"os"
)

func readProfile(filename string) (*pb.MatchObject, *pb.MmfConfig, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file \"%s\": %s", filename, err.Error())
	}
	defer jsonFile.Close()

	// parse json data and remove extra whitespace before sending to the backend.
	jsonData, _ := ioutil.ReadAll(jsonFile) // this reads as a byte array
	buffer := new(bytes.Buffer)             // convert byte array to buffer to send to json.Compact()
	if err := json.Compact(buffer, jsonData); err != nil {
		dirLog.WithError(err).WithField("filename", filename).Warn("error compacting profile json")
	}

	jsonProfile := buffer.String()

	profileName := "fallback-name"
	if gjson.Get(jsonProfile, "name").Exists() {
		profileName = gjson.Get(jsonProfile, "name").String()
	}

	pbProfile := &pb.MatchObject{
		Id:         profileName,
		Properties: jsonProfile,
	}

	mmfcfg := &pb.MmfConfig{Name: "profileName"}
	mmfcfg.Type = pb.MmfConfig_GRPC
	mmfcfg.Host = gjson.Get(jsonProfile, "hostname").String()
	mmfcfg.Port = int32(gjson.Get(jsonProfile, "port").Int())

	return pbProfile, mmfcfg, nil
}

func collectPlayerIds(rosters []*pb.Roster) (ids []string) {
	for _, r := range rosters {
		for _, p := range r.Players {
			if p.Id != "" {
				ids = append(ids, p.Id)
			}
		}
	}
	return
}
