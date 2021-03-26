const scfToken = artifacts.require("SCFToken");

module.exports = function (deployer) {
  deployer.deploy(scfToken);
};
