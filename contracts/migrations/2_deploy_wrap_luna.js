const scfToken = artifacts.require("ScfToken");

module.exports = function (deployer) {
  deployer.deploy(scfToken);
};
