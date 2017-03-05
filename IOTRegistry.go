/*
Copyright (c) 2016 Skuchain,Inc

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"crypto/sha256"

	"github.com/btcsuite/btcd/btcec"
	proto "github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/zmanian/IOTRegistry/IOTRegistryStore"
	IOTRegistryTX "github.com/zmanian/IOTRegistry/IOTRegistryTX"
)

type IOTRegistry struct {
}

/*
	Init is a required function in which necessary setup operations are performed.
	In this case, no such operations are needed.
*/
func (t *IOTRegistry) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	return nil, nil
}

/*
	verifies an input signature against input public key and message.
*/
func verify(pubKeyBytes []byte, sigBytes []byte, message string) (err error) {
	//deserialize public key bytes into a public key object
	creatorKey, err := btcec.ParsePubKey(pubKeyBytes, btcec.S256())
	if err != nil {
		fmt.Printf("Invalid pubkey: (%s)\n", hex.EncodeToString(pubKeyBytes))
		return fmt.Errorf("Invalid pubkey key (%s)\n", hex.EncodeToString(pubKeyBytes))
	}

	//DER is a standard for serialization
	//parsing DER signature from bitcoin curve into a signature object
	signature, err := btcec.ParseDERSignature(sigBytes, btcec.S256())
	if err != nil {
		fmt.Printf("Bad Creator signature encoding\n")
		return fmt.Errorf("Bad Creator signature encoding\n")
	}

	messageBytes := sha256.Sum256([]byte(message))

	//try to verify the signature
	success := signature.Verify(messageBytes[:], creatorKey)
	if !success {
		fmt.Printf("Invalid Creator Signature\n")
		return fmt.Errorf("Invalid Creator Signature\n")
	}
	return nil
}

/*
	Invoke is the central mechanism in hyperledger for creating transactions and putting them to the ledger.
	This function takes as arguments
	|		a chaincode interface, called "stub"
	|		a string which dictates which function to perform,
	|		a slice of strings "args" that accord with some protobuf specification
*/
func (t *IOTRegistry) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if len(args) == 0 {
		fmt.Printf("Insufficient arguments found\n")
		return nil, fmt.Errorf("Insufficient arguments found\n")
	}
	argsBytes, err := hex.DecodeString(args[0])
	if err != nil {
		fmt.Printf("Invalid argument (%s) expected hex\n", args[0])
		return nil, fmt.Errorf("Invalid argument (%s) expected hex\n", args[0])
	}

	switch function {
	/*
		registerOwner puts an "OwnerName: <OwnerName>" state to the ledger, indexed by the ownerName.
		TX struct: 		RegisterOwnerTX
		Store struct: 	Owner
	*/
	case "registerOwner":
		//declare and initialize RegisterIdentity struct
		registerNameArgs := IOTRegistryTX.RegisterOwnerTX{}
		err = proto.Unmarshal(argsBytes, &registerNameArgs)
		if err != nil {
			fmt.Printf("Invalid argument expected RegisterNameTX protocol buffer %s\n", err.Error())
			return nil, fmt.Errorf("Invalid argument expected RegisterNameTX protocol buffer %s\n", err.Error())
		}

		if len(registerNameArgs.OwnerName) == 0 {
			fmt.Printf("length of OwnerName (%s) is zero\n", registerNameArgs.OwnerName)
			return nil, fmt.Errorf("length of OwnerName (%s) is zero\n", registerNameArgs.OwnerName)
		}
		if len(registerNameArgs.PubKey) == 0 {
			fmt.Printf("length of Pubkey (%s) is zero\n", registerNameArgs.PubKey)
			return nil, fmt.Errorf("length of Pubkey (%s) is zero\n", registerNameArgs.PubKey)
		}
		if len(registerNameArgs.Signature) == 0 {
			fmt.Printf("length of Signature (%s) is zero\n", registerNameArgs.Signature)
			return nil, fmt.Errorf("length of Signature (%s) is zero\n", registerNameArgs.Signature)
		}
		//check if name is available
		registerNameBytes, err := stub.GetState("OwnerName: " + registerNameArgs.OwnerName)
		if err != nil {
			fmt.Printf("Could not get OwnerName (%s) State\n", registerNameArgs.OwnerName)
			return nil, fmt.Errorf("Could not get OwnerName (%s) State\n", registerNameArgs.OwnerName)
		}

		//if name unavailable
		if len(registerNameBytes) != 0 {
			fmt.Printf("OwnerName (%s) is unavailable\n", registerNameArgs.OwnerName)
			return nil, fmt.Errorf("OwnerName (%s) is unavailable\n", registerNameArgs.OwnerName)
		}

		creatorKeyBytes := registerNameArgs.PubKey
		creatorSig := registerNameArgs.Signature
		message := registerNameArgs.OwnerName + ":" + registerNameArgs.Data

		err = verify(creatorKeyBytes, creatorSig, message)
		if err != nil {
			fmt.Printf("Error verifying signature (%s)\n", creatorSig)
			return nil, fmt.Errorf("Error verifying signature (%s)\n", creatorSig)
		}

		//marshall into store type. Then put that variable into the state
		store := IOTRegistryStore.Owner{}
		store.OwnerName = registerNameArgs.OwnerName
		store.Pubkey = registerNameArgs.PubKey
		storeBytes, err := proto.Marshal(&store)
		if err != nil {
			fmt.Printf("Error marshalling variable of type IOTRegistryStore.Identities{}: (%v)\n", err.Error())
			return nil, fmt.Errorf("Error marshalling variable of type IOTRegistryStore.Identities{}: (%v)\n", err.Error())
		}

		err = stub.PutState("OwnerName: "+registerNameArgs.OwnerName, storeBytes)
		if err != nil {
			fmt.Printf("error putting OwnerName (%s) to ledger: (%v)\n", registerNameArgs.OwnerName, err.Error())
			return nil, fmt.Errorf("error putting OwnerName (%s) to ledger: (%v)\n", registerNameArgs.OwnerName, err.Error())
		}
	/*
		registerThing does, essentially, two things.
		1.	puts a "Thing: <Nonce>" state to the ledger, indexed by the nonce.
		|		-a thing contains a string slice of identities, an OwnerName, an arbitrary string of data, and the name of a specification.
		2.	for each element of the Identities string slice, puts an "Alias: <identity>" state to the ledger, indexed by identity.
		|		-an Alias contains a nonce, which can be used to access its parent "thing"
		TX struct: 		RegisterThingTX
		Store structs: 	Thing, Alias
	*/
	case "registerThing":
		registerThingArgs := IOTRegistryTX.RegisterThingTX{}
		err = proto.Unmarshal(argsBytes, &registerThingArgs)
		if err != nil {
			fmt.Printf("Invalid argument expected RegisterThingTX protocol buffer. Err: (%s)\n", err.Error())
			return nil, fmt.Errorf("Invalid argument expected RegisterThingTX protocol buffer. Err: (%s)\n", err.Error())
		}
		if len(registerThingArgs.OwnerName) == 0 {
			fmt.Printf("length of OwnerName (%s) is zero\n", registerThingArgs.OwnerName)
			return nil, fmt.Errorf("length of OwnerName (%s) is zero\n", registerThingArgs.OwnerName)
		}
		if len(registerThingArgs.Nonce) == 0 {
			fmt.Printf("length of Nonce (%s) is zero\n", registerThingArgs.Nonce)
			return nil, fmt.Errorf("length of Nonce (%s) is zero\n", registerThingArgs.Nonce)
		}
		if len(registerThingArgs.Signature) == 0 {
			fmt.Printf("length of Signature (%s) is zero\n", registerThingArgs.Signature)
			return nil, fmt.Errorf("length of Signature (%s) is zero\n", registerThingArgs.Signature)
		}

		//check if nonce already exists
		nonceCheckBytes, err := stub.GetState("Thing: " + hex.EncodeToString(registerThingArgs.Nonce))
		if err != nil {
			fmt.Printf("Could not get Nonce (%s) State\n", hex.EncodeToString(registerThingArgs.Nonce))
			return nil, fmt.Errorf("Could not get Nonce (%s) State\n", hex.EncodeToString(registerThingArgs.Nonce))
		}

		//if nonce exists
		if len(nonceCheckBytes) != 0 {
			fmt.Printf("Nonce (%s) is unavailable\n", hex.EncodeToString(registerThingArgs.Nonce))
			return nil, fmt.Errorf("Nonce (%s) is unavailable\n", hex.EncodeToString(registerThingArgs.Nonce))
		}

		//check if owner is valid id (name exists in registry)
		checkIDBytes, err := stub.GetState("OwnerName: " + registerThingArgs.OwnerName)
		if err != nil {
			fmt.Printf("Failed to look up OwnerName (%s) \n", registerThingArgs.OwnerName)
			return nil, fmt.Errorf("Failed to look up OwnerName (%s) \n", registerThingArgs.OwnerName)
		}

		//if owner is not registered name
		if len(checkIDBytes) == 0 {
			fmt.Printf("OwnerName (%s) is not registered\n", registerThingArgs.OwnerName)
			return nil, fmt.Errorf("OwnerName (%s) is not registered\n", registerThingArgs.OwnerName)
		}

		//check if any identities exist
		//we're checking if any identities are registered as ownernames but not if they are registered as aliases
		for _, identity := range registerThingArgs.Identities {
			aliasCheckBytes, err := stub.GetState("OwnerName: " + identity)
			if err != nil {
				fmt.Printf("Could not get identity: (%s) State\n", identity)
				return nil, fmt.Errorf("Could not get identity: (%s) State\n", identity)
			}
			//throw error if any of the identities already exist
			if len(aliasCheckBytes) != 0 {
				fmt.Printf("Ownername: (%s) is already in registry\n", identity)
				return nil, fmt.Errorf("Ownername: (%s) is already in registry\n", identity)
			}
		}

		//retrieve state associated with owner name to get public key
		ownerRegistration := IOTRegistryStore.Owner{}
		err = proto.Unmarshal(checkIDBytes, &ownerRegistration)
		if err != nil {
			fmt.Printf("Error unmarshalling OwnerName (%s) state (%v)", registerThingArgs.OwnerName, err.Error())
			return nil, fmt.Errorf("Error unmarshalling OwnerName (%s) state (%v)", registerThingArgs.OwnerName, err.Error())
		}

		ownerPubKeyBytes := ownerRegistration.Pubkey

		ownerSig := registerThingArgs.Signature

		//TODO review later
		message := registerThingArgs.OwnerName
		for _, identity := range registerThingArgs.Identities {
			message += ":" + identity
		}
		message += ":" + registerThingArgs.Data
		message += ":" + registerThingArgs.Spec
		err = verify(ownerPubKeyBytes, ownerSig, message)
		if err != nil {
			fmt.Printf("Error verifying signature (%s)", ownerSig)
			return nil, fmt.Errorf("Error verifying signature (%s)", ownerSig)
		}

		for _, identity := range registerThingArgs.Identities {

			alias := IOTRegistryStore.Alias{}
			alias.Nonce = registerThingArgs.Nonce
			aliasStoreBytes, err := proto.Marshal(&alias)

			if err != nil {
				fmt.Printf("Error marshalling alias (%v) into bytes", alias)
				return nil, fmt.Errorf("Error marshalling alias (%v) into bytes\n", alias)
			}
			stub.PutState("Alias: "+identity, aliasStoreBytes)
		}

		store := IOTRegistryStore.Thing{}
		store.Alias = registerThingArgs.Identities
		store.OwnerName = registerThingArgs.OwnerName
		store.Data = registerThingArgs.Data
		store.SpecName = registerThingArgs.Spec
		storeBytes, err := proto.Marshal(&store)
		if err != nil {
			fmt.Printf("error marshalling type IOTRegistry store :(%v)\n", err.Error())
			return nil, fmt.Errorf("error marshalling type IOTRegistry store :(%v)\n", err.Error())
		}
		err = stub.PutState("Thing: "+hex.EncodeToString(registerThingArgs.Nonce), storeBytes)
		if err != nil {
			fmt.Printf("Error putting thing state :(%v)", err.Error())
			return nil, fmt.Errorf("Error putting thing state :(%v)", err.Error())
		}
	/*
		registerSpec puts a "Spec: <SpecName>" state to the ledger, indexed by the spec name.
		TX struct: 		RegisterSpecTX
		Store structs: 	Spec
	*/
	case "registerSpec":
		specArgs := IOTRegistryTX.RegisterSpecTX{}
		err = proto.Unmarshal(argsBytes, &specArgs)
		if err != nil {
			fmt.Printf("Invalid argument expected RegisterSpecTX protocol buffer %s\n", err.Error())
		}

		if len(specArgs.OwnerName) == 0 {
			return nil, fmt.Errorf("length of OwnerName (%s) is zero\n", specArgs.OwnerName)
		}
		if len(specArgs.SpecName) == 0 {
			return nil, fmt.Errorf("length of Nonce (%s) is zero\n", specArgs.SpecName)
		}
		if len(specArgs.Signature) == 0 {
			return nil, fmt.Errorf("length of Signature (%s) is zero\n", specArgs.Signature)
		}

		//check if spec already exists
		specNameCheckBytes, err := stub.GetState("Spec: " + specArgs.SpecName)
		if err != nil {
			fmt.Printf("Could not get Spec State\n")
			return nil, fmt.Errorf("Could not get Spec State\n")
		}

		//if spec already exists
		if len(specNameCheckBytes) != 0 {
			fmt.Printf("SpecName (%s) is unavailable\n", specArgs.SpecName)
			return nil, fmt.Errorf("SpecName (%s) is unavailable\n", specArgs.SpecName)
		}

		//check if owner is valid id (name exists in registry)
		checkIDBytes, err := stub.GetState("OwnerName: " + specArgs.OwnerName)
		if err != nil {
			fmt.Printf("Failed to look up OwnerName\n")
			return nil, fmt.Errorf("Failed to look up OwnerName (%s)\n", specArgs.OwnerName)
		}

		//if owner is not registered name
		if len(checkIDBytes) == 0 {
			fmt.Printf("OwnerName is not registered\n")
			return nil, fmt.Errorf("OwnerName is not registered %s\n", specArgs.OwnerName)
		}

		//retrieve state associated with owner name to get public key
		ownerRegistration := IOTRegistryStore.Owner{}
		err = proto.Unmarshal(checkIDBytes, &ownerRegistration)
		if err != nil {
			return nil, err
		}

		ownerPubKeyBytes := ownerRegistration.Pubkey

		ownerSig := specArgs.Signature

		//TODO review later
		message := specArgs.SpecName + ":" + specArgs.OwnerName + ":" + specArgs.Data
		err = verify(ownerPubKeyBytes, ownerSig, message)
		if err != nil {
			return nil, fmt.Errorf("Error verifying signature\n")
		}

		store := IOTRegistryStore.Spec{}
		store.OwnerName = specArgs.OwnerName
		store.Data = specArgs.Data
		storeBytes, err := proto.Marshal(&store)
		if err != nil {
			fmt.Println(err)
		}
		err = stub.PutState("Spec: "+specArgs.SpecName, storeBytes)
		if err != nil {
			fmt.Printf(err.Error())
			return nil, err
		}
	}
	return nil, nil
}

/* declares, initializes, and marshalls struct containing owner information to JSON */
func ownerNameToJSON(ownerName string, pubKey []byte) ([]byte, error) {
	type JSONIdentities struct {
		OwnerName string
		Pubkey    string
	}
	jsonOwner := JSONIdentities{}
	jsonOwner.OwnerName = ownerName
	jsonOwner.Pubkey = hex.EncodeToString(pubKey)

	jsonstring, err := json.Marshal(jsonOwner)
	if err != nil {
		return nil, err
	}
	return jsonstring, nil
}

/*
	Query is a mechanism for requesting information from the ledger. There are three query methods in this chaincode: owner, thing, and spec.
	Each query will return data as a json formatted slice of bytes
*/
func (t *IOTRegistry) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	// fmt.Printf("function: %s\n", function)
	switch function {
	/*
		An "owner" query requests information stored in the ledger about a particular owner.
		If the owner is registered, the JSON will contain the owner's name and public key.
	*/
	case "owner":
		if len(args) != 1 {
			return nil, fmt.Errorf("No argument specified\n")
		}

		owner := IOTRegistryStore.Owner{}

		ownerName := args[0]
		ownerBytes, err := stub.GetState("OwnerName: " + ownerName)
		if err != nil {
			fmt.Printf(err.Error())
			return nil, err
		}

		if len(ownerBytes) == 0 {
			return nil, fmt.Errorf("OwnerName (%s) does not exist\n", ownerName)
		}
		err = proto.Unmarshal(ownerBytes, &owner)
		if err != nil {
			fmt.Printf(err.Error())
			return nil, err
		}
		jsonBytes, err := ownerNameToJSON(owner.OwnerName, owner.Pubkey)
		return jsonBytes, err
		/*
			A "thing" query requests information stored in the ledger about a particular thing.
			Things are indexed by a Nonce, which should be a valid hex string.
			If the thing is registered, the JSON will contain the owner's list of aliases, owner name, an arbitrary string of data, and a spec name.
		*/
	case "thing":
		if len(args) != 1 {
			return nil, fmt.Errorf("No argument specified\n")
		}
		thing := IOTRegistryStore.Thing{}
		thingNonce := args[0]
		thingBytes, err := stub.GetState("Thing: " + thingNonce)
		if err != nil {
			fmt.Printf(err.Error())
			return nil, err
		}

		if len(thingBytes) == 0 {
			return nil, fmt.Errorf("Thing (%s) does not exist\n", thingNonce)
		}

		err = proto.Unmarshal(thingBytes, &thing)
		if err != nil {
			fmt.Printf(err.Error())
			return nil, err
		}
		return json.Marshal(thing)
		/*
			A "spec" query requests information stored in the ledger about a particular specification.
			Specs are indexed by a SpecName, which is a string.
			If the spec is registered, the JSON will contain the owner's name and a string of data.
		*/
	case "spec":
		if len(args) != 1 {
			return nil, fmt.Errorf("no argument specified\n")
		}

		spec := IOTRegistryStore.Spec{}
		specName := args[0]

		specBytes, err := stub.GetState("Spec: " + specName)
		if err != nil {
			fmt.Printf(err.Error())
			return nil, err
		}

		if len(specBytes) == 0 {
			return nil, fmt.Errorf("spec (%s) does not exist\n", specName)
		}

		err = proto.Unmarshal(specBytes, &spec)
		if err != nil {
			fmt.Printf(err.Error())
			return nil, err
		}
		return json.Marshal(spec)
	}
	return nil, nil
}

func main() {
	err := shim.Start(new(IOTRegistry))
	if err != nil {
		fmt.Printf("Error starting chaincode: %s\n", err)
	}
}
