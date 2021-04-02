// SPDX-License-Identifier: MIT

import "https://github.com/QuarkChain/quarkchain-native-token/blob/master/contracts/NativeToken.sol";

pragma solidity ^0.6.0;

contract Burner is AllowNonDefaultNativeToken {
    
    event Burn(uint256 tokenId, uint256 amount, address source);
    
    function burn() public payable allowToken {
        require(msg.value > 0);
        uint256 tokenId = NativeToken.getCurrentToken();
        emit Burn(tokenId, msg.value, msg.sender);
    }
    
}