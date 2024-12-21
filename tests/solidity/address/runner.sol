// SPDX-License-Identifier: MIT
pragma solidity >=0.6.0 <0.8.0;

contract TestContract {
    uint256 public value;

    constructor(uint256 _value) {
        value = _value;
    }
}

contract AddressChecker {
    // Mapping to store addresses
    mapping(address => bool) private addressMapping;

    // Array to store addresses
    address[] private addressArray;

    // Nested mapping for advanced storage
    mapping(address => mapping(string => uint256)) private data;

    // Whitelist example
    mapping(address => bool) private whitelist;
    // Blacklist example
    mapping(address => bool) private blacklist;


    // Owner address
    address public owner;

    uint256 public initialBalance;

    uint256 public testDelNumber; // Storage to modify via delegatecall

    // Test deploy contracts
    address[] public deployedContracts;


    // Events
    event AddressAdded(address indexed addr);
    event SenderChecked(address indexed sender);
    event EtherSent(address indexed from, address indexed to, uint256 amount);
    event currentOwner(address indexed currentOwner);
    event DelegateCalled(address indexed caller, uint256 newValue);
    event CodeHashRetrieved(address indexed addr, bytes32 codeHash);
    event ContractDeployed(address indexed newContract, uint256 value);


    struct SimpleData {
        address userAddress;
        uint256 data1;
        uint256 data2;
    }

    SimpleData[] private simpleDataList;

    constructor() payable {
        owner = msg.sender; // Set deployer as owner
        require(msg.value > 0, "Must send Ether during deployment");
        initialBalance = msg.value;
    }

    receive() external payable {}

    // Function to log and return the address of the sender
    function checkSender() public returns (address) {
        emit SenderChecked(msg.sender); // Log the sender's address
        return msg.sender;             // Return the sender's address
    }

    // Basic Functions
    function addToMapping(address _addr) public {
        require(_addr != address(0), "Invalid address addToMapping");
        addressMapping[_addr] = true;
    }

    function isInMapping(address _addr) public view returns (bool) {
        return addressMapping[_addr];
    }

    function addToArray(address _addr) public {
        require(_addr != address(0), "Invalid address addToArray");
        addressArray.push(_addr);
    }

    // Function to set a new owner
    function setOwner(address _newOwner) public {
        require(_newOwner != address(0), "New owner cannot be the zero address");
        owner = _newOwner;
        emit currentOwner(owner); // Log owner
    }

    function getOwner() public view returns (address) {
        return owner;
    }

    // Function to send Ether
    function sendEther(address payable _to, uint256 _amount) public {
        require(address(this).balance >= _amount, "Insufficient balance in contract, deploy the checker contract with at least 1 ether");
        require(_to != address(0), "Invalid recipient address");

        _to.transfer(_amount);
        emit EtherSent(msg.sender, _to, _amount);
    }

    function getBalance() public view returns (uint256) {
        return address(this).balance;
    }

    function isInArray(address _addr) public view returns (bool) {
        for (uint256 i = 0; i < addressArray.length; i++) {
            if (addressArray[i] == _addr) {
                return true;
            }
        }
        return false;
    }

    // Function to test simple function call by address
    function addressFuncCall() public {
        emit SenderChecked(msg.sender);
    }

    // Function to update the state of the calling contract
    function setNumber(uint256 _number) public {
        testDelNumber = _number; // Updates the `testDelNumber` variable in the caller's storage
        emit DelegateCalled(msg.sender, _number);
    }

    function validateAddress(address _addr) public pure returns (bool) {
        return _addr != address(0);
    }

    function getAddressAtIndex(uint256 index) public view returns (address) {
        require(index < addressArray.length, "Index out of bounds");
        return addressArray[index];
    }

    function getAddressArrayLength() public view returns (uint256) {
        return addressArray.length;
    }

    // Nested Mapping Functions
    function setAddressData(address _addr, string memory key, uint256 value) public {
        data[_addr][key] = value;
    }

    function getAddressData(address _addr, string memory key) public view returns (uint256) {
        return data[_addr][key];
    }

    // Whitelist Functions
    function addToWhitelist(address _addr) public {
        whitelist[_addr] = true;
    }

    function isWhitelisted(address _addr) public view returns (bool) {
        return whitelist[_addr];
    }

    // Blacklist Functions
    function addToBlacklist(address _addr) public {
        blacklist[_addr] = true;
    }

    function isBlacklisted(address _addr) public view returns (bool) {
        return blacklist[_addr];
    }

    // Logging Events
    function logAddress(address _addr) public {
        require(_addr != address(0), "Invalid address logAddress");
        emit AddressAdded(_addr);
    }

    // Advanced Address Operations
    function isContract(address _addr) public view returns (bool) {
        uint256 size;
        assembly {
            size := extcodesize(_addr)
        }
        return size > 0;
    }

    function getAddressBalance(address _addr) public view returns (uint256) {
        return _addr.balance;
    }

    // Address Comparison
    function compareAddresses(address addr1, address addr2) public pure returns (bool) {
        return addr1 == addr2;
    }

    // Index Mapping for Unique Addresses
    mapping(address => uint256) private addressIndex;

    function addUniqueAddress(address _addr) public {
        require(_addr != address(0), "Invalid address addUniqueAddress");
        if (addressIndex[_addr] == 0) {
            addressArray.push(_addr);
            addressIndex[_addr] = addressArray.length; // Index starts at 1
        }
    }

    function getIndex(address _addr) public view returns (uint256) {
        return addressIndex[_addr];
    }

    // Add simple data
    function addSimpleData(address _addr, uint256 _data1, uint256 _data2) public {
        simpleDataList.push(SimpleData({userAddress: _addr, data1: _data1, data2: _data2}));
    }

    // Get simple data
    function getSimpleData(uint256 index) public view returns (address, uint256, uint256) {
        require(index < simpleDataList.length, "Index out of bounds");
        SimpleData memory simpleDataEntry = simpleDataList[index];
        return (simpleDataEntry.userAddress, simpleDataEntry.data1, simpleDataEntry.data2);
    }

    // Function to retrieve the code hash of an address
    function getCodeHash(address _addr) public returns (bytes32) {
        bytes32 codeHash;
        assembly {
            codeHash := extcodehash(_addr)
        }

        emit CodeHashRetrieved(_addr, codeHash);

        return codeHash;
    }

    // Deploy a new contract and store its address
    function deployNewContract(uint256 _value) public {
        TestContract newContract = new TestContract(_value);
        deployedContracts.push(address(newContract));
        emit ContractDeployed(address(newContract), _value);
    }

    // Get the total number of deployed contracts
    function getDeployedContractsCount() public view returns (uint256) {
        return deployedContracts.length;
    }

    // Get the address of a deployed contract by index
    function getDeployedContract(uint256 index) public view returns (address) {
        require(index < deployedContracts.length, "Index out of bounds");
        return deployedContracts[index];
    }

}

contract Runner {
    AddressChecker private addressChecker;
    uint256 public testDelNumber; // Storage to be updated via delegatecall

    event TestCompleted(string testName, string message);
    event SenderVerified(address indexed sender);
    event ContractBalances(string step, uint256 checkerBalance, uint256 testerBalance);
    event DelegateCallExecuted(address indexed caller, uint256 newValue);
    event DeployedContractsCount(uint256 count);
    event DeployedContractAddress(uint256 index, address contractAddress);

    address testAddress1 = 0x0000000000000000000000000000000000000000000000000000000000001000;
    address testAddress2 = 0x0000000000000000000000000000000000000000000000000000000000002000;

    function runAllTests(address payable _addressChecker) public {
        require(_addressChecker != address(0), "Invalid AddressChecker contract address");
        addressChecker = AddressChecker(_addressChecker);

        runTestSet1();
        runTestSet2();
        runTestSet3();
    }

    function runTestSet1() public {
        // Check the message sender
        testCheckSender();
        emit TestCompleted("testCheckSender", "Success");

        // Test adding to mapping
        bool mappingTestResult = testAddToMapping(testAddress1);
        emit TestCompleted("testAddToMapping", mappingTestResult ? "Success" : "Failure");

        // Test checking mapping
        bool isInMapping = testCheckMapping(testAddress1);
        emit TestCompleted("testCheckMapping", isInMapping ? "Success" : "Failure");

        // Test adding to array
        bool arrayTestResult = testAddToArray(testAddress1);
        emit TestCompleted("testAddToArray", arrayTestResult ? "Success" : "Failure");

        // Test checking array
        bool isInArray = testCheckArray(testAddress1);
        emit TestCompleted("testCheckArray", isInArray ? "Success" : "Failure");

        // Test the owner setting and verification
        address testOwner = testSetAndCheckOwner(testAddress1); 
        emit TestCompleted("testSetAndCheckOwner", testOwner == testAddress1 ? "Success" : "Failure");

        // Call the testSendEther function
        uint256 testAmount = 1 ether;
        testSendEther(testAmount); // Call the Ether-sending test
        emit TestCompleted("testSendEther", "Success");
    }

    function runTestSet2() public {
        // Test function call with address
        testAddressCall(address(addressChecker));
        emit TestCompleted("testAddressCall", "Success");

        // Test the delegatecall functionality
        testDelegateCall(10); 
        emit TestCompleted("testDelegateCall", "Success");

        // Test validating address
        bool validateResult = testValidateAddress(testAddress2);
        emit TestCompleted("testValidateAddress", validateResult ? "Valid" : "Invalid");

        // Test unique address
        uint256 uniqueIndex = testUniqueAddress(testAddress1);
        emit TestCompleted("testUniqueAddress", uniqueIndex > 0 ? "Success" : "Failure");

        // Test nested mapping
        string memory key = "testKey";
        uint256 value = 42;
        bool nestedMappingTestResult = testNestedMapping(testAddress1, key, value);
        emit TestCompleted("testNestedMapping", nestedMappingTestResult ? "Success" : "Failure");

        // Test whitelist
        bool whitelistTestResult = testWhitelist(testAddress1);
        emit TestCompleted("testWhitelist", whitelistTestResult ? "Whitelisted" : "Not Whitelisted");

        // Test blacklist
        bool blacklistTestResult = testBlacklist(testAddress2);
        emit TestCompleted("testBlacklist", blacklistTestResult ? "Blacklisted" : "Not Blacklisted");

        // Test logging address
        testLogAddress(testAddress2);
        emit TestCompleted("testLogAddress", "Logged Successfully");

        // Test checking if address is a contract
        bool isContractTestResult = testIsContract(testAddress1);
        emit TestCompleted("testIsContract testAddress1", isContractTestResult ? "Is Contract" : "Is Not Contract");
        isContractTestResult = testIsContract(address(addressChecker));
        emit TestCompleted("testIsContract addressChecker", isContractTestResult ? "Is Contract" : "Is Not Contract");

        // Test adding simple struct data
        bool simpleDataTestResult = testSimpleStruct(testAddress1, 42, 84);
        emit TestCompleted("testSimpleStruct", simpleDataTestResult ? "Success" : "Failure");
    }

    function runTestSet3() public {
        // Test retriving of code hash
        testCodeHash(address(addressChecker));

        // Test deploying a new contract
        testDeployNewContract(500); 
        emit TestCompleted("testDeployNewContract", "Success");

        // Test getting deployed contracts count
        testGetDeployedContractsCount(); 
        emit TestCompleted("testGetDeployedContractsCount", "Success");

        // Test retrieving a deployed contract
        testGetDeployedContract(0); 
        emit TestCompleted("testGetDeployedContract", "Success");
    }

    // Test: Check and log the sender address
    function testCheckSender() public {
        address sender = addressChecker.checkSender(); // Call checker's function
        emit SenderVerified(sender);                  // Log the returned sender
    }

    // Test: Add to mapping
    function testAddToMapping(address _addr) public returns (bool) {
        addressChecker.addToMapping(_addr);
        return addressChecker.isInMapping(_addr);
    }

    // Test: Check if address exists in mapping
    function testCheckMapping(address _addr) public view returns (bool) {
        return addressChecker.isInMapping(_addr);
    }

    // Test: Add to array
    function testAddToArray(address _addr) public returns (bool) {
        addressChecker.addToArray(_addr);
        return addressChecker.isInArray(_addr);
    }

    // Test: Check if address exists in array
    function testCheckArray(address _addr) public view returns (bool) {
        return addressChecker.isInArray(_addr);
    }

    // Test setting and verifying the owner
    function testSetAndCheckOwner(address _newOwner) public returns (address) {
        // Set a new owner
        addressChecker.setOwner(_newOwner);

        // Verify that the new owner is correctly set
        return addressChecker.getOwner();
    }

    // Test the sendEther function in AddressChecker
    function testSendEther(uint256 _amount) public {
        address payable recipient = payable(address(this)); // Tester contract as the recipient
        
        // Log balances before the transaction
        emit ContractBalances(
            "Before Transaction",
            address(addressChecker).balance,
            address(this).balance
        );

        addressChecker.sendEther(recipient, _amount);      // Call the checker contract

        // Log balances after the transaction
        emit ContractBalances(
            "After Transaction",
            address(addressChecker).balance,
            address(this).balance
        );
    }

    // Get the runner contract balance
    function getTesterBalance() public view returns (uint256) {
        return address(this).balance;
    }

    // Receive Ether
    receive() external payable {}

    // Test function call by address
    function testAddressCall(address _addr) public {
        (bool success, ) = _addr.call(
            abi.encodeWithSignature("addressFuncCall()")
        );
        require(success, "Address call failed");
    }

    // Test the delegatecall functionality
    function testDelegateCall(uint256 _number) public {
        (bool success, ) = address(addressChecker).delegatecall(
            abi.encodeWithSignature("setNumber(uint256)", _number)
        );

        require(success, "Delegatecall failed");
        emit DelegateCallExecuted(msg.sender, testDelNumber); // Log the new value
    }

    // Test: Validate address
    function testValidateAddress(address _addr) public view returns (bool) {
        return addressChecker.validateAddress(_addr);
    }

    // Test: Add unique address
    function testUniqueAddress(address _addr) public returns (uint256) {
        addressChecker.addUniqueAddress(_addr);
        return addressChecker.getIndex(_addr);
    }

    // Test: Nested mapping
    function testNestedMapping(address _addr, string memory key, uint256 value) public returns (bool) {
        addressChecker.setAddressData(_addr, key, value);
        uint256 storedValue = addressChecker.getAddressData(_addr, key);
        return storedValue == value;
    }

    // Test: Whitelist
    function testWhitelist(address _addr) public returns (bool) {
        addressChecker.addToWhitelist(_addr);
        return addressChecker.isWhitelisted(_addr);
    }

    // Test: Blacklist
    function testBlacklist(address _addr) public returns (bool) {
        addressChecker.addToBlacklist(_addr);
        return addressChecker.isBlacklisted(_addr);
    }

    // Test: Log an address
    function testLogAddress(address _addr) public {
        addressChecker.logAddress(_addr);
    }

    // Test: Check if an address is a contract
    function testIsContract(address _addr) public view returns (bool) {
        return addressChecker.isContract(_addr);
    }

    function getRunCaller() public view returns (address) {
        return address(this);
    }

    function getCheckerOwner() public view returns (address) {
        return addressChecker.getOwner();
    }

    // Test: Add and retrieve a simple struct
    function testSimpleStruct(address _addr, uint256 data1, uint256 data2) public returns (bool) {
        addressChecker.addSimpleData(_addr, data1, data2);

        // Retrieve the struct data
        (address retrievedAddr, uint256 retrievedData1, uint256 retrievedData2) = addressChecker.getSimpleData(0);

        // Verify the struct data
        return (retrievedAddr == _addr && retrievedData1 == data1 && retrievedData2 == data2);
    }

    function testCodeHash(address _addr) public {
        bytes32 codeHash = addressChecker.getCodeHash(_addr);

        // Verify if the code hash matches expectations
        if (codeHash == bytes32(0)) {
            emit TestCompleted("testCodeHash", "Failed");
        } else {
            emit TestCompleted("testCodeHash", "Success");
        }
    }

    // Test deploying a new contract
    function testDeployNewContract(uint256 _value) public {
        addressChecker.deployNewContract(_value);
    }

    // Test retrieving the total number of deployed contracts
    function testGetDeployedContractsCount() public {
        uint256 count = addressChecker.getDeployedContractsCount();
        emit DeployedContractsCount(count);
    }

    // Test retrieving the address of a deployed contract
    function testGetDeployedContract(uint256 index) public {
        address contractAddress = addressChecker.getDeployedContract(index);
        emit DeployedContractAddress(index, contractAddress);
    }
}
