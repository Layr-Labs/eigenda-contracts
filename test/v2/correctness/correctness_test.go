package correctness

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Layr-Labs/eigenda/api/clients/codecs"
	"github.com/Layr-Labs/eigenda/api/clients/v2"
	"github.com/Layr-Labs/eigenda/api/clients/v2/coretypes"
	"github.com/Layr-Labs/eigenda/core"
	auth "github.com/Layr-Labs/eigenda/core/auth/v2"
	"github.com/Layr-Labs/eigenda/encoding"
	"github.com/Layr-Labs/eigenda/encoding/utils/codec"
	"github.com/Layr-Labs/eigenda/test/v2/client"
	"github.com/docker/go-units"

	"github.com/Layr-Labs/eigenda/common/testutils/random"
	"github.com/stretchr/testify/require"
)

// A list of config files that this test runs against
var environments = []string{
	client.PreprodEnv,
	client.TestnetEnv,
}

// getEnvironmentName takes an environment string as listed in environments (aka a path to a config file describing
// the environment) and returns the name of the environment. Assumes the path is in the format of
// "path/to/ENVIRONMENT_NAME.json".
func getEnvironmentName(environment string) string {
	elements := strings.Split(environment, "/")
	fileName := elements[len(elements)-1]
	environmentName := strings.Split(fileName, ".")[0]
	return environmentName
}

// Tests the basic dispersal workflow:
// - disperse a blob
// - wait for it to be confirmed
// - read the blob from the relays
// - read the blob from the validators
func testBasicDispersal(
	t *testing.T,
	c *client.TestClient,
	payload []byte,
	certVerifierAddress string,
) error {
	if certVerifierAddress == "" {
		t.Skip("Requested cert verifier address is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err := c.DisperseAndVerify(ctx, certVerifierAddress, payload)
	if err != nil {
		return fmt.Errorf("failed to disperse and verify: %v", err)
	}

	return nil
}

// Disperse a 0 byte blob.
// Empty blobs are not allowed by the disperser
func emptyBlobDispersalTest(t *testing.T, environment string) {
	blobBytes := []byte{}
	quorums := []core.QuorumID{0, 1}

	c := client.GetTestClient(t, environment)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// We have to use the disperser client directly, since it's not possible for the PayloadDisperser to
	// attempt dispersal of an empty blob
	// This should fail with "data is empty" error
	_, _, err := c.GetDisperserClient().DisperseBlob(ctx, blobBytes, 0, quorums)
	require.Error(t, err)
	require.ErrorContains(t, err, "blob size must be greater than 0")
}

func TestEmptyBlobDispersal(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			emptyBlobDispersalTest(t, environment)
		})
	}
}

// Disperse an empty payload. Blob will not be empty, since payload encoding entails adding bytes
func emptyPayloadDispersalTest(t *testing.T, environment string) {
	payload := []byte{}

	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	c := client.GetTestClient(t, environment)

	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1)
	require.NoError(t, err)
}

func TestEmptyPayloadDispersal(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			emptyPayloadDispersalTest(t, environment)
		})
	}
}

// Disperse a payload that consists only of 0 bytes
func testZeroPayloadDispersalTest(t *testing.T, environment string) {
	payload := make([]byte, 1000)

	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	c := client.GetTestClient(t, environment)

	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1)
	require.NoError(t, err)
}

func TestZeroPayloadDispersal(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			testZeroPayloadDispersalTest(t, environment)
		})
	}
}

// Disperse a blob that consists only of 0 bytes. This should be permitted by eigenDA, even
// though it's not permitted by the default payload -> blob encoding scheme
func zeroBlobDispersalTest(t *testing.T, environment string) {
	blobBytes := make([]byte, 1000)
	quorums := []core.QuorumID{0, 1}

	c := client.GetTestClient(t, environment)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// We have to use the disperser client directly, since it's not possible for the PayloadDisperser to
	// attempt dispersal of a blob containing all 0s
	_, _, err := c.GetDisperserClient().DisperseBlob(ctx, blobBytes, 0, quorums)
	require.NoError(t, err)
}

func TestZeroBlobDispersal(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			zeroBlobDispersalTest(t, environment)
		})
	}
}

// Disperse a 1 byte payload (no padding).
func microscopicBlobDispersalTest(t *testing.T, environment string) {
	payload := []byte{1}

	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	c := client.GetTestClient(t, environment)

	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1)
	require.NoError(t, err)
}

func TestMicroscopicBlobDispersal(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			microscopicBlobDispersalTest(t, environment)
		})
	}
}

// Disperse a 1 byte payload (with padding).
func microscopicBlobDispersalWithPadding(t *testing.T, environment string) {
	payload := []byte{1}

	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	c := client.GetTestClient(t, environment)

	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1)
	require.NoError(t, err)
}

func TestMicroscopicBlobDispersalWithPadding(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			microscopicBlobDispersalWithPadding(t, environment)
		})
	}
}

// Disperse a small payload (between 1KB and 2KB).
func smallBlobDispersalTest(t *testing.T, environment string) {
	rand := random.NewTestRandom()
	payload := rand.VariableBytes(units.KiB, 2*units.KiB)

	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	c := client.GetTestClient(t, environment)

	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1)
	require.NoError(t, err)
}

func TestSmallBlobDispersal(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			smallBlobDispersalTest(t, environment)
		})
	}
}

// Disperse a medium payload (between 100KB and 200KB).
func mediumBlobDispersalTest(t *testing.T, environment string) {
	rand := random.NewTestRandom()
	payload := rand.VariableBytes(100*units.KiB, 200*units.KiB)

	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	c := client.GetTestClient(t, environment)

	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1)
	require.NoError(t, err)
}

func TestMediumBlobDispersal(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			mediumBlobDispersalTest(t, environment)
		})
	}
}

// Disperse a medium payload (between 1MB and 2MB).
func largeBlobDispersalTest(t *testing.T, environment string) {
	rand := random.NewTestRandom()

	config, err := client.GetConfig(environment)
	require.NoError(t, err)
	maxBlobSize := int(config.MaxBlobSize)

	payload := rand.VariableBytes(maxBlobSize/2, maxBlobSize*3/4)

	c := client.GetTestClient(t, environment)

	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1)
	require.NoError(t, err)
}

func TestLargeBlobDispersal(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			largeBlobDispersalTest(t, environment)
		})
	}
}

// Disperse a small payload (between 1KB and 2KB) with each of the defined quorum sets available
func smallBlobDispersalAllQuorumsSetsTest(t *testing.T, environment string) {
	rand := random.NewTestRandom()
	payload := rand.VariableBytes(units.KiB, 2*units.KiB)

	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	c := client.GetTestClient(t, environment)

	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1)
	require.NoError(t, err)
	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1_2)
	require.NoError(t, err)
	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums2)
	require.NoError(t, err)
}

func TestSmallBlobDispersalAllQuorumsSets(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			smallBlobDispersalAllQuorumsSetsTest(t, environment)
		})
	}
}

// Disperse a blob that is exactly at the maximum size after padding (16MB)
func maximumSizedBlobDispersalTest(t *testing.T, environment string) {
	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	maxPermissibleDataLength, err := codec.GetMaxPermissiblePayloadLength(
		uint32(config.MaxBlobSize) / encoding.BYTES_PER_SYMBOL)
	require.NoError(t, err)

	rand := random.NewTestRandom()
	payload := rand.Bytes(int(maxPermissibleDataLength))

	c := client.GetTestClient(t, environment)

	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1)
	require.NoError(t, err)
}

func TestMaximumSizedBlobDispersal(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			maximumSizedBlobDispersalTest(t, environment)
		})
	}
}

// Disperse a blob that is too large (>16MB after padding)
func tooLargeBlobDispersalTest(t *testing.T, environment string) {
	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	maxPermissibleDataLength, err := codec.GetMaxPermissiblePayloadLength(uint32(config.MaxBlobSize) / encoding.BYTES_PER_SYMBOL)
	require.NoError(t, err)

	rand := random.NewTestRandom()
	payload := rand.Bytes(int(maxPermissibleDataLength) + 1)

	c := client.GetTestClient(t, environment)

	err = testBasicDispersal(t, c, payload, config.EigenDACertVerifierAddressQuorums0_1)
	require.Error(t, err)
}

func TestTooLargeBlobDispersal(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			tooLargeBlobDispersalTest(t, environment)
		})
	}
}

func doubleDispersalTest(t *testing.T, environment string) {
	rand := random.NewTestRandom()
	c := client.GetTestClient(t, environment)

	payload := rand.VariableBytes(units.KiB, 2*units.KiB)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	err = c.DisperseAndVerify(ctx, config.EigenDACertVerifierAddressQuorums0_1, payload)
	require.NoError(t, err)

	// disperse again
	err = c.DisperseAndVerify(ctx, config.EigenDACertVerifierAddressQuorums0_1, payload)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "blob already exists"))
}

func TestDoubleDispersal(t *testing.T) {
	t.Skip("This test is not working ever since we removed the salt param from the top level client.")

	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			doubleDispersalTest(t, environment)
		})
	}
}

func unauthorizedGetChunksTest(t *testing.T, environment string) {
	rand := random.NewTestRandom()
	c := client.GetTestClient(t, environment)
	config, err := client.GetConfig(environment)
	require.NoError(t, err)

	payload := rand.VariableBytes(units.KiB, 2*units.KiB)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	eigenDACert, err := c.DispersePayload(ctx, config.EigenDACertVerifierAddressQuorums0_1, payload)
	require.NoError(t, err)

	blobKey, err := eigenDACert.ComputeBlobKey()
	require.NoError(t, err)

	targetRelay := eigenDACert.BlobInclusionInfo.BlobCertificate.RelayKeys[0]

	chunkRequests := make([]*clients.ChunkRequestByRange, 1)
	chunkRequests[0] = &clients.ChunkRequestByRange{
		BlobKey: *blobKey,
		Start:   0,
		End:     1,
	}
	_, err = c.GetRelayClient().GetChunksByRange(ctx, targetRelay, chunkRequests)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get operator key: operator not found")
}

func TestUnauthorizedGetChunks(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			unauthorizedGetChunksTest(t, environment)
		})
	}
}

func dispersalWithInvalidSignatureTest(t *testing.T, environment string) {
	quorums := []core.QuorumID{0, 1}

	rand := random.NewTestRandom()

	c := client.GetTestClient(t, environment)

	// Create a dispersal client with a random key
	signer, err := auth.NewLocalBlobRequestSigner(fmt.Sprintf("%x", rand.Bytes(32)))
	require.NoError(t, err)

	accountId, err := signer.GetAccountID()
	require.NoError(t, err)
	fmt.Printf("Account ID: %s\n", accountId.Hex())

	disperserConfig := &clients.DisperserClientConfig{
		Hostname:          c.GetConfig().DisperserHostname,
		Port:              fmt.Sprintf("%d", c.GetConfig().DisperserPort),
		UseSecureGrpcFlag: true,
	}
	disperserClient, err := clients.NewDisperserClient(disperserConfig, signer, nil, nil)
	require.NoError(t, err)

	payloadBytes := rand.VariableBytes(units.KiB, 2*units.KiB)

	payload := coretypes.NewPayload(payloadBytes)

	// TODO (litt3): make the blob form configurable. Using PolynomialFormCoeff means that the data isn't being
	//  FFTed/IFFTed, and it is important for both modes of operation to be tested.
	blob, err := payload.ToBlob(codecs.PolynomialFormCoeff)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, _, err = disperserClient.DisperseBlob(ctx, blob.Serialize(), 0, quorums)
	require.Error(t, err)
	require.Contains(t, err.Error(), "error accounting blob")
}

func TestDispersalWithInvalidSignature(t *testing.T) {
	for _, environment := range environments {
		t.Run(getEnvironmentName(environment), func(t *testing.T) {
			dispersalWithInvalidSignatureTest(t, environment)
		})
	}
}
