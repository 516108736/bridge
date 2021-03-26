// SPDX-License-Identifier: MIT
pragma solidity >=0.6.0;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract WrappedToken is ERC20, Ownable {

    constructor(string memory name, string memory symbol)
    public
    ERC20(name, symbol)
    {}


    function mint(address account, uint256 amount) public onlyOwner {
        _mint(account, amount);
    }
}

contract SCFToken is WrappedToken {
    constructor() public WrappedToken("Scf111", "scfscf") {}
}