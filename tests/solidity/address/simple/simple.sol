// SPDX-License-Identifier: MIT
pragma solidity >=0.6.0 <0.8.0;

contract Simple {

    address testAddress1 = 0xa000000000000000000000000000000000000000000000000000000000001000;

    function getAddress() public view returns(address) {
        return testAddress1;
    }

    function getInputAddress(address input) public pure returns(address) {
        return input;
    }
}