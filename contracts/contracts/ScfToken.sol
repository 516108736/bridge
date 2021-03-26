// SPDX-License-Identifier: MIT
pragma solidity >=0.6.0;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract ScfToken is ERC20{
    
    constructor ()  ERC20("SSCCFF","SCF") {
        _mint(msg.sender,100000*(10**uint256(decimals())));
    }
}