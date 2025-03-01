// SPDX-License-Identifier: MIT
pragma solidity >=0.6.0 <0.8.0;

interface IERC20 {
  function totalSupply() external view returns (uint256);
  function balanceOf(address who) external view returns (uint256);
  function allowance(address owner, address spender) external view returns (uint256);
  function transfer(address to, uint256 value) external returns (bool);
  function approve(address spender, uint256 value) external returns (bool);
  function transferFrom(address from, address to, uint256 value) external returns (bool);

  event Transfer(address indexed from, address indexed to, uint256 value);
  event Approval(address indexed owner, address indexed spender, uint256 value);
}

library SafeMath {
  function mul(uint256 a, uint256 b) internal pure returns (uint256) {
    if (a == 0) {
      return 0;
    }
    uint256 c = a * b;
    assert(c / a == b);
    return c;
  }

  function div(uint256 a, uint256 b) internal pure returns (uint256) {
    uint256 c = a / b;
    return c;
  }

  function sub(uint256 a, uint256 b) internal pure returns (uint256) {
    assert(b <= a);
    return a - b;
  }

  function add(uint256 a, uint256 b) internal pure returns (uint256) {
    uint256 c = a + b;
    assert(c >= a);
    return c;
  }

  function ceil(uint256 a, uint256 m) internal pure returns (uint256) {
    uint256 c = add(a,m);
    uint256 d = sub(c,1);
    return mul(div(d,m),m);
  }
}

contract TokenDetailed is IERC20 {

  using SafeMath for uint256;
  mapping (address => uint256) private _balances;
  mapping (address => mapping (address => uint256)) private _allowed;

  uint256 private _totalSupply;
  uint256 private _basePercent = 100;
  uint256 private _baseBurnPercentDivisor;

  string private _name;
  string private _symbol;
  uint8 private _decimals;

  address private _owner;


  event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

  constructor
  (
    string memory tokenName,
    string memory tokenSymbol,
    uint256 tokenTotalSupply,
    uint256 baseBurnPercentDivisor,
    uint8 tokenDecimals
  ) {
    _name = tokenName;
    _symbol = tokenSymbol;
    _totalSupply = tokenTotalSupply;
    _decimals = tokenDecimals;
    _baseBurnPercentDivisor = baseBurnPercentDivisor;
  }

  function name() public view returns(string memory) {
    return _name;
  }

  function symbol() public view returns(string memory) {
    return _symbol;
  }

  function decimals() public view returns(uint8) {
    return _decimals;
  }

  function totalSupply() public view virtual override returns (uint256) {
    return _totalSupply;
  }

  function balanceOf(address accountOwner) public view virtual override returns (uint256) {
    return _balances[accountOwner];
  }

  function allowance(address accountOwner, address spender) public view virtual override returns (uint256) {
    return _allowed[accountOwner][spender];
  }

  //This function calculates number of tokens to burn, given an input number of tokens
  function calculateNumTokensToBurn(uint256 numTokens) public view returns (uint256)  {
    uint256 roundValue = numTokens.ceil(_basePercent);
    return roundValue.mul(_basePercent).div(_baseBurnPercentDivisor);
  }

  function transfer(address to, uint256 value) public virtual override returns (bool) {
    require(value <= _balances[msg.sender]);

    uint256 tokensToBurn = calculateNumTokensToBurn(value);
    uint256 tokensToTransfer = value.sub(tokensToBurn);

    _balances[msg.sender] = _balances[msg.sender].sub(value);
    _balances[to] = _balances[to].add(tokensToTransfer);

    _totalSupply = _totalSupply.sub(tokensToBurn);

    emit Transfer(msg.sender, to, tokensToTransfer);
    emit Transfer(msg.sender, address(0), tokensToBurn);

    return true;
  }

  function approve(address spender, uint256 value) public virtual override returns (bool) {
    require(spender != address(0));

    _allowed[msg.sender][spender] = value;
    emit Approval(msg.sender, spender, value);
    return true;
  }

  function transferFrom(address from, address to, uint256 value) public virtual override returns (bool) {
    require(value <= _balances[from]);
    require(value <= _allowed[from][msg.sender]);

    _balances[from] = _balances[from].sub(value);

    uint256 tokensToBurn = calculateNumTokensToBurn(value);
    uint256 tokensToTransfer = value.sub(tokensToBurn);

    _balances[to] = _balances[to].add(tokensToTransfer);
    _totalSupply = _totalSupply.sub(tokensToBurn);

    _allowed[from][msg.sender] = _allowed[from][msg.sender].sub(value);

    emit Transfer(from, to, tokensToTransfer);
    emit Transfer(from, address(0), tokensToBurn);

    return true;
  }

  function increaseAllowance(address spender, uint256 addedValue) public returns (bool) {
    require(spender != address(0));
    _allowed[msg.sender][spender] = (_allowed[msg.sender][spender].add(addedValue));
    emit Approval(msg.sender, spender, _allowed[msg.sender][spender]);
    return true;
  }

  function decreaseAllowance(address spender, uint256 subtractedValue) public returns (bool) {
    require(spender != address(0));
    _allowed[msg.sender][spender] = (_allowed[msg.sender][spender].sub(subtractedValue));
    emit Approval(msg.sender, spender, _allowed[msg.sender][spender]);
    return true;
  }

  function _mint(address account, uint256 amount) internal {
    require(amount != 0);
    _owner = account;
    _balances[account] = _balances[account].add(amount);
    emit Transfer(address(0), account, amount);
  }

  function _burn(address account, uint256 amount) internal {
    require(amount != 0);
    require(amount <= _balances[account]);
    _totalSupply = _totalSupply.sub(amount);
    _balances[account] = _balances[account].sub(amount);
    emit Transfer(account, address(0), amount);
  }

  function multiTransfer(address[] memory receivers, uint256[] memory amounts) public {
    for (uint256 i = 0; i < receivers.length; i++) {
      transfer(receivers[i], amounts[i]);
    }
  }

  function _checkOwner() internal view virtual {
    if (owner() != msg.sender) {
      revert ("sender is not onwer");
    }
  }

  function owner() public view virtual returns (address) {
    return _owner;
  }

  modifier onlyOwner() {
    _checkOwner();
    _;
  }

  function transferOwnership(address newOwner) public virtual onlyOwner {
    if (newOwner == address(0)) {
      revert ("OwnableInvalidOwner");
    }
    _transferOwnership(newOwner);
  }

  function renounceOwnership() public virtual onlyOwner {
    _transferOwnership(address(0));
  }

  function _transferOwnership(address newOwner) internal virtual {
    address oldOwner = _owner;
    _owner = newOwner;
    emit OwnershipTransferred(oldOwner, newOwner);
  }
}

/*
  An example test token demonstrating usage (for testing only)
*/
contract Y2Q is TokenDetailed {

  string constant tokenNameWeNeed = "Year2Quantum";
  string constant tokenSymbol = "Y2Q";
  uint8 decimalsWeNeed = 18;

  uint256 totalSupplyWeNeed = 100 * (10**12) * (10**decimalsWeNeed);
  uint256  baseBurnPercentDivisor = 100000; //0.1% per transaction

  constructor() payable TokenDetailed
  (
  tokenNameWeNeed,
  tokenSymbol,
  totalSupplyWeNeed,
  baseBurnPercentDivisor,
  decimalsWeNeed
  )
  {
    _mint(msg.sender, totalSupply());
  }

}
