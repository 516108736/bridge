const burner = artifacts.require("Burner");

module.exports = function (deployer) {
  deployer.deploy(burner);
};
