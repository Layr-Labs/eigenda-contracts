// SPDX-License-Identifier: BUSL-1.1
pragma solidity =0.8.12;

import {EmptyContract} from "../lib/eigenlayer-middleware/lib/eigenlayer-contracts/src/test/mocks/EmptyContract.sol";
import {EjectionManager} from "../lib/eigenlayer-middleware/src/EjectionManager.sol";
import {IEjectionManager} from "../lib/eigenlayer-middleware/src/interfaces/IEjectionManager.sol";
import {RegistryCoordinator} from "../lib/eigenlayer-middleware/src/RegistryCoordinator.sol";
import {IRegistryCoordinator} from "../lib/eigenlayer-middleware/src/interfaces/IRegistryCoordinator.sol";
import {StakeRegistry} from "../lib/eigenlayer-middleware/src/StakeRegistry.sol";
import {IStakeRegistry} from "../lib/eigenlayer-middleware/src/interfaces/IStakeRegistry.sol";
import "@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol";

import "forge-std/Test.sol";
import "forge-std/Script.sol";
import "forge-std/StdJson.sol";

contract Deployer_EjectionManager is Script, Test {
    string public existingDeploymentInfoPath =
        string(bytes("./script/deploy/mainnet/output/mainnet_deployment_data.json"));
    string public deployConfigPath = string(bytes("./script/deploy/mainnet/config/ejector.config.json"));

    address ejectorOwner;
    address ejector;
    address deployer;

    EjectionManager public ejectionManager;
    EjectionManager public ejectionManagerImplementation;

    RegistryCoordinator public registryCoordinator;
    StakeRegistry public stakeRegistry;
    ProxyAdmin public eigenDAProxyAdmin;
    EmptyContract public emptyContract;

    function run() external {
        string memory existingDeploymentData = vm.readFile(existingDeploymentInfoPath);

        eigenDAProxyAdmin = ProxyAdmin(stdJson.readAddress(existingDeploymentData, ".addresses.eigenDAProxyAdmin"));
        registryCoordinator =
            RegistryCoordinator(stdJson.readAddress(existingDeploymentData, ".addresses.registryCoordinator"));
        stakeRegistry = StakeRegistry(stdJson.readAddress(existingDeploymentData, ".addresses.stakeRegistry"));

        string memory config_data = vm.readFile(deployConfigPath);

        uint256 currentChainId = block.chainid;
        uint256 configChainId = stdJson.readUint(config_data, ".chainInfo.chainId");
        emit log_named_uint("You are deploying on ChainID", currentChainId);
        require(configChainId == currentChainId, "You are on the wrong chain for this config");

        ejectorOwner = stdJson.readAddress(config_data, ".permissions.owner");
        ejector = stdJson.readAddress(config_data, ".permissions.ejector");
        deployer = stdJson.readAddress(config_data, ".permissions.deployer");

        emptyContract = EmptyContract(stdJson.readAddress(config_data, ".permissions.emptyContract"));

        vm.startBroadcast();

        ejectionManager =
            EjectionManager(address(new TransparentUpgradeableProxy(address(emptyContract), address(deployer), "")));

        ejectionManagerImplementation = new EjectionManager(registryCoordinator, stakeRegistry);

        IEjectionManager.QuorumEjectionParams[] memory quorumEjectionParams = _parseQuorumEjectionParams(config_data);
        address[] memory ejectors = new address[](1);
        ejectors[0] = ejector;

        TransparentUpgradeableProxy(payable(address(ejectionManager))).upgradeToAndCall(
            address(ejectionManagerImplementation),
            abi.encodeWithSelector(EjectionManager.initialize.selector, ejectorOwner, ejectors, quorumEjectionParams)
        );

        TransparentUpgradeableProxy(payable(address(ejectionManager))).changeAdmin(address(eigenDAProxyAdmin));

        vm.stopBroadcast();

        console.log("EjectionManager deployed at: ", address(ejectionManager));
        console.log("EjectionManagerImplementation deployed at: ", address(ejectionManagerImplementation));

        _sanityCheck(ejectionManager, ejectionManagerImplementation, config_data);
    }

    function _sanityCheck(
        EjectionManager _ejectionManager,
        EjectionManager _ejectionManagerImplementation,
        string memory config_data
    ) internal {
        require(
            address(_ejectionManager.registryCoordinator()) == address(registryCoordinator),
            "ejectionManager.registryCoordinator() != registryCoordinator"
        );
        require(
            address(_ejectionManager.stakeRegistry()) == address(stakeRegistry),
            "ejectionManager.stakeRegistry() != stakeRegistry"
        );
        require(
            address(_ejectionManagerImplementation.registryCoordinator()) == address(registryCoordinator),
            "ejectionManagerImplementation.registryCoordinator() != registryCoordinator"
        );
        require(
            address(_ejectionManagerImplementation.stakeRegistry()) == address(stakeRegistry),
            "ejectionManagerImplementation.stakeRegistry() != stakeRegistry"
        );

        require(
            eigenDAProxyAdmin.getProxyImplementation(TransparentUpgradeableProxy(payable(address(_ejectionManager))))
                == address(_ejectionManagerImplementation),
            "ejectionManager: implementation set incorrectly"
        );

        require(_ejectionManager.owner() == ejectorOwner, "ejectionManager.owner() != ejectorOwner");
        require(_ejectionManager.isEjector(ejector) == true, "ejector != ejector");

        IEjectionManager.QuorumEjectionParams[] memory quorumEjectionParams = _parseQuorumEjectionParams(config_data);
        for (uint8 i = 0; i < quorumEjectionParams.length; ++i) {
            (uint32 rateLimitWindow, uint16 ejectableStakePercent) = _ejectionManager.quorumEjectionParams(i);
            IEjectionManager.QuorumEjectionParams memory params =
                IEjectionManager.QuorumEjectionParams(rateLimitWindow, ejectableStakePercent);
            require(
                keccak256(abi.encode(params)) == keccak256(abi.encode(quorumEjectionParams[i])),
                "ejectionManager.quorumEjectionParams != quorumEjectionParams"
            );
        }
    }

    function _parseQuorumEjectionParams(string memory config_data)
        internal
        returns (IEjectionManager.QuorumEjectionParams[] memory quorumEjectionParams)
    {
        bytes memory quorumEjectionParamsRaw = stdJson.parseRaw(config_data, ".quorumEjectionParams");
        quorumEjectionParams = abi.decode(quorumEjectionParamsRaw, (IEjectionManager.QuorumEjectionParams[]));
    }
}
