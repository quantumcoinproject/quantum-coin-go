// SPDX-License-Identifier: MIT
pragma solidity >=0.6.0 <0.8.0;
pragma abicoder v2;

interface ITokenConversionContract {
    event OnRequestConversion(
        address indexed quantumAddress,
        string ethAddress,
        string ethereumSignature,
        uint256 index
    );

    event OnSubmitBurnProof(
        address indexed submitterAddress,
        string indexed burnProof,
        uint256 index
    );
}

contract TokenConversionContract is ITokenConversionContract {

    struct ConversionRequest {
        address quantumAddress;
        string ethAddress;
        string ethSignature;
    }

    ConversionRequest[] public ConversionRequests;
    string[] public BurnProofs;

    function requestConversion(string calldata ethAddress, string calldata ethSignature) external returns (uint8) {
        ConversionRequests.push(ConversionRequest(msg.sender, ethAddress, ethSignature)); //just a request, anyone can request
        emit OnRequestConversion(msg.sender, ethAddress, ethSignature, ConversionRequests.length - 1);
        return 0;
    }

    function submitBurnProof(string calldata burnProof) external returns (uint8) {
        BurnProofs.push(burnProof); //anyone can submit a burn proof
        emit OnSubmitBurnProof(msg.sender, burnProof, BurnProofs.length - 1);
        return 0;
    }
}