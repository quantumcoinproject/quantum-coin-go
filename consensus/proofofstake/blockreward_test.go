package proofofstake

import (
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

var blockYears = 4

var blockRewardTotal = []float64{951293.759512938, 475646.879756469, 237823.439878234, 118911.719939117, 59455.8599695586, 29727.9299847793, 14863.9649923897, 7431.98249619483,
	3715.99124809741, 1857.99562404871, 928.997812024353, 464.498906012177, 232.249453006088, 116.124726503044, 58.0623632515221, 29.031181625761,
	14.5155908128805, 7.25779540644026, 3.62889770322013, 1.81444885161006, 0.907224425805032, 0.453612212902516, 0.226806106451258, 0.113403053225629,
	0.0567015266128145, 0.0283507633064073, 0.0141753816532036, 0.00708769082660182, 0.00354384541330091, 0.00177192270665045, 0.000885961353325227,
	0.000442980676662613, 0.000221490338331307, 0.000110745169165653, 0.0000553725845828267, 0.0000276862922914133, 0.0000138431461457067,
	0.00000692157307285334, 0.00000346078653642667, 0.00000173039326821333, 0.000000865196634106667, 0.000000432598317053333, 0.000000216299158526667,
	0.000000108149579263333, 0.0000000540747896316667, 0.0000000270373948158333, 0.0000000135186974079167, 0.00000000675934870395834, 0.00000000337967435197917,
	0.00000000168983717598958, 0.000000000844918587994792, 0.000000000422459293997396, 0.000000000211229646998698, 0.000000000105614823499349,
	5.28074117496745e-11, 2.64037058748373e-11, 1.32018529374186e-11, 6.60092646870931e-12, 3.30046323435466e-12, 1.65023161717733e-12,
	8.25115808588664e-13, 4.12557904294332e-13, 2.06278952147166e-13, 1.03139476073583e-13, 5.15697380367915e-14, 2.57848690183957e-14,
	1.28924345091979e-14, 6.44621725459894e-15, 3.22310862729947e-15, 1.61155431364973e-15, 8.05777156824867e-16, 4.02888578412434e-16,
	2.01444289206217e-16, 1.00722144603108e-16, 5.03610723015542e-17, 2.51805361507771e-17}

var blockStartRang = int64(1497600)

var blockEndRange = []int64{22521600, 43545600, 64569600, 85593600, 106617600, 127641600, 148665600, 169689600, 190713600, 211737600, 232761600, 253785600,
	274809600, 295833600, 316857600, 337881600, 358905600, 379929600, 400953600, 421977600, 443001600, 464025600, 485049600, 506073600,
	527097600, 548121600, 569145600, 590169600, 611193600, 632217600, 653241600, 674265600, 695289600, 716313600, 737337600, 758361600,
	779385600, 800409600, 821433600, 842457600, 863481600, 884505600, 905529600, 926553600, 947577600, 968601600, 989625600, 1010649600,
	1031673600, 1052697600, 1073721600, 1094745600, 1115769600, 1136793600, 1157817600, 1178841600, 1199865600, 1220889600, 1241913600,
	1262937600, 1283961600, 1304985600, 1326009600, 1347033600, 1368057600, 1389081600, 1410105600, 1431129600, 1452153600, 1473177600,
	1494201600, 1515225600, 1536249600, 1557273600, 1578297600, 1599321600}

func TestRewardGenerateYearly(t *testing.T) {
	fmt.Println("rewardStartBlock===", rewardStartBlock)
	for i := 1; i <= 350; i++ {
		blockNumber := rewardStartBlock.Int64() + (blockYearly.Int64() * int64(i))
		startBlockNumber := big.NewInt(blockNumber - blockYearly.Int64())
		startReward := new(big.Int).Set(GetReward(startBlockNumber))

		endBlockNumber := big.NewInt(blockNumber - 1)
		endReward := new(big.Int).Set(GetReward(endBlockNumber))

		startRewardEth := params.WeiToEther(startReward)

		fmt.Println("In Wei", "Year : ", i,
			" Block Range : ", startBlockNumber, " - ", endBlockNumber,
			" Block reward range (wei) : ", startReward, " - ", endReward,
			" Block reward range (q) : ", startRewardEth, " - ", params.WeiToEther(endReward))

		if i == 1 && startRewardEth.Cmp(big.NewInt(951293)) != 0 {
			t.Fatalf("failed a")
		}
		if i == 5 && startRewardEth.Cmp(big.NewInt(475646)) != 0 {
			t.Fatalf("failed b")
		}
		if i == 9 && startRewardEth.Cmp(big.NewInt(237823)) != 0 {
			t.Fatalf("failed c")
		}
		if i == 13 && startRewardEth.Cmp(big.NewInt(118911)) != 0 {
			t.Fatalf("failed d")
		}
		if i == 17 && startRewardEth.Cmp(big.NewInt(59455)) != 0 {
			t.Fatalf("failed e")
		}
		if i == 21 && startRewardEth.Cmp(big.NewInt(29727)) != 0 {
			t.Fatalf("failed f")
		}
		if i == 25 && startRewardEth.Cmp(big.NewInt(14863)) != 0 {
			t.Fatalf("failed g")
		}
		if i == 29 && startRewardEth.Cmp(big.NewInt(7431)) != 0 {
			t.Fatalf("failed h")
		}
		if i == 33 && startRewardEth.Cmp(big.NewInt(3715)) != 0 {
			t.Fatalf("failed i")
		}
		if i == 37 && startRewardEth.Cmp(big.NewInt(1857)) != 0 {
			t.Fatalf("failed i")
		}
		if i == 41 && startRewardEth.Cmp(big.NewInt(928)) != 0 {
			t.Fatalf("failed i")
		}
		if i == 45 && startRewardEth.Cmp(big.NewInt(464)) != 0 {
			t.Fatalf("failed i")
		}
		if i == 49 && startRewardEth.Cmp(big.NewInt(232)) != 0 {
			t.Fatalf("failed i")
		}
		if i == 53 && startRewardEth.Cmp(big.NewInt(116)) != 0 {
			t.Fatalf("failed i")
		}
		if i == 57 && startRewardEth.Cmp(big.NewInt(58)) != 0 {
			t.Fatalf("failed i")
		}
	}
	/*
		In Wei Year :  1  Block Range :  277204  -  5533203  Block reward range (wei) :  951293759512937627732754  -  951293759512937627732754  Block reward range (q) :  951293  -  951293
		In Wei Year :  2  Block Range :  5533204  -  10789203  Block reward range (wei) :  951293759512937627732754  -  951293759512937627732754  Block reward range (q) :  951293  -  951293
		In Wei Year :  3  Block Range :  10789204  -  16045203  Block reward range (wei) :  951293759512937627732754  -  951293759512937627732754  Block reward range (q) :  951293  -  951293
		In Wei Year :  4  Block Range :  16045204  -  21301203  Block reward range (wei) :  951293759512937627732754  -  951293759512937627732754  Block reward range (q) :  951293  -  951293
		In Wei Year :  5  Block Range :  21301204  -  26557203  Block reward range (wei) :  475646879756468813866377  -  475646879756468813866377  Block reward range (q) :  475646  -  475646
		In Wei Year :  6  Block Range :  26557204  -  31813203  Block reward range (wei) :  475646879756468813866377  -  475646879756468813866377  Block reward range (q) :  475646  -  475646
		In Wei Year :  7  Block Range :  31813204  -  37069203  Block reward range (wei) :  475646879756468813866377  -  475646879756468813866377  Block reward range (q) :  475646  -  475646
		In Wei Year :  8  Block Range :  37069204  -  42325203  Block reward range (wei) :  475646879756468813866377  -  475646879756468813866377  Block reward range (q) :  475646  -  475646
		In Wei Year :  9  Block Range :  42325204  -  47581203  Block reward range (wei) :  237823439878234406933188  -  237823439878234406933188  Block reward range (q) :  237823  -  237823
		In Wei Year :  10  Block Range :  47581204  -  52837203  Block reward range (wei) :  237823439878234406933188  -  237823439878234406933188  Block reward range (q) :  237823  -  237823
		In Wei Year :  11  Block Range :  52837204  -  58093203  Block reward range (wei) :  237823439878234406933188  -  237823439878234406933188  Block reward range (q) :  237823  -  237823
		In Wei Year :  12  Block Range :  58093204  -  63349203  Block reward range (wei) :  237823439878234406933188  -  237823439878234406933188  Block reward range (q) :  237823  -  237823
		In Wei Year :  13  Block Range :  63349204  -  68605203  Block reward range (wei) :  118911719939117203466594  -  118911719939117203466594  Block reward range (q) :  118911  -  118911
		In Wei Year :  14  Block Range :  68605204  -  73861203  Block reward range (wei) :  118911719939117203466594  -  118911719939117203466594  Block reward range (q) :  118911  -  118911
		In Wei Year :  15  Block Range :  73861204  -  79117203  Block reward range (wei) :  118911719939117203466594  -  118911719939117203466594  Block reward range (q) :  118911  -  118911
		In Wei Year :  16  Block Range :  79117204  -  84373203  Block reward range (wei) :  118911719939117203466594  -  118911719939117203466594  Block reward range (q) :  118911  -  118911
		In Wei Year :  17  Block Range :  84373204  -  89629203  Block reward range (wei) :  59455859969558601733297  -  59455859969558601733297  Block reward range (q) :  59455  -  59455
		In Wei Year :  18  Block Range :  89629204  -  94885203  Block reward range (wei) :  59455859969558601733297  -  59455859969558601733297  Block reward range (q) :  59455  -  59455
		In Wei Year :  19  Block Range :  94885204  -  100141203  Block reward range (wei) :  59455859969558601733297  -  59455859969558601733297  Block reward range (q) :  59455  -  59455
		In Wei Year :  20  Block Range :  100141204  -  105397203  Block reward range (wei) :  59455859969558601733297  -  59455859969558601733297  Block reward range (q) :  59455  -  59455
		In Wei Year :  21  Block Range :  105397204  -  110653203  Block reward range (wei) :  29727929984779300866649  -  29727929984779300866649  Block reward range (q) :  29727  -  29727
		In Wei Year :  22  Block Range :  110653204  -  115909203  Block reward range (wei) :  29727929984779300866649  -  29727929984779300866649  Block reward range (q) :  29727  -  29727
		In Wei Year :  23  Block Range :  115909204  -  121165203  Block reward range (wei) :  29727929984779300866649  -  29727929984779300866649  Block reward range (q) :  29727  -  29727
		In Wei Year :  24  Block Range :  121165204  -  126421203  Block reward range (wei) :  29727929984779300866649  -  29727929984779300866649  Block reward range (q) :  29727  -  29727
		In Wei Year :  25  Block Range :  126421204  -  131677203  Block reward range (wei) :  14863964992389650433324  -  14863964992389650433324  Block reward range (q) :  14863  -  14863
		In Wei Year :  26  Block Range :  131677204  -  136933203  Block reward range (wei) :  14863964992389650433324  -  14863964992389650433324  Block reward range (q) :  14863  -  14863
		In Wei Year :  27  Block Range :  136933204  -  142189203  Block reward range (wei) :  14863964992389650433324  -  14863964992389650433324  Block reward range (q) :  14863  -  14863
		In Wei Year :  28  Block Range :  142189204  -  147445203  Block reward range (wei) :  14863964992389650433324  -  14863964992389650433324  Block reward range (q) :  14863  -  14863
		In Wei Year :  29  Block Range :  147445204  -  152701203  Block reward range (wei) :  7431982496194825216662  -  7431982496194825216662  Block reward range (q) :  7431  -  7431
		In Wei Year :  30  Block Range :  152701204  -  157957203  Block reward range (wei) :  7431982496194825216662  -  7431982496194825216662  Block reward range (q) :  7431  -  7431
		In Wei Year :  31  Block Range :  157957204  -  163213203  Block reward range (wei) :  7431982496194825216662  -  7431982496194825216662  Block reward range (q) :  7431  -  7431
		In Wei Year :  32  Block Range :  163213204  -  168469203  Block reward range (wei) :  7431982496194825216662  -  7431982496194825216662  Block reward range (q) :  7431  -  7431
		In Wei Year :  33  Block Range :  168469204  -  173725203  Block reward range (wei) :  3715991248097412608331  -  3715991248097412608331  Block reward range (q) :  3715  -  3715
		In Wei Year :  34  Block Range :  173725204  -  178981203  Block reward range (wei) :  3715991248097412608331  -  3715991248097412608331  Block reward range (q) :  3715  -  3715
		In Wei Year :  35  Block Range :  178981204  -  184237203  Block reward range (wei) :  3715991248097412608331  -  3715991248097412608331  Block reward range (q) :  3715  -  3715
		In Wei Year :  36  Block Range :  184237204  -  189493203  Block reward range (wei) :  3715991248097412608331  -  3715991248097412608331  Block reward range (q) :  3715  -  3715
		In Wei Year :  37  Block Range :  189493204  -  194749203  Block reward range (wei) :  1857995624048706304166  -  1857995624048706304166  Block reward range (q) :  1857  -  1857
		In Wei Year :  38  Block Range :  194749204  -  200005203  Block reward range (wei) :  1857995624048706304166  -  1857995624048706304166  Block reward range (q) :  1857  -  1857
		In Wei Year :  39  Block Range :  200005204  -  205261203  Block reward range (wei) :  1857995624048706304166  -  1857995624048706304166  Block reward range (q) :  1857  -  1857
		In Wei Year :  40  Block Range :  205261204  -  210517203  Block reward range (wei) :  1857995624048706304166  -  1857995624048706304166  Block reward range (q) :  1857  -  1857
		In Wei Year :  41  Block Range :  210517204  -  215773203  Block reward range (wei) :  928997812024353152083  -  928997812024353152083  Block reward range (q) :  928  -  928
		In Wei Year :  42  Block Range :  215773204  -  221029203  Block reward range (wei) :  928997812024353152083  -  928997812024353152083  Block reward range (q) :  928  -  928
		In Wei Year :  43  Block Range :  221029204  -  226285203  Block reward range (wei) :  928997812024353152083  -  928997812024353152083  Block reward range (q) :  928  -  928
		In Wei Year :  44  Block Range :  226285204  -  231541203  Block reward range (wei) :  928997812024353152083  -  928997812024353152083  Block reward range (q) :  928  -  928
		In Wei Year :  45  Block Range :  231541204  -  236797203  Block reward range (wei) :  464498906012176576041  -  464498906012176576041  Block reward range (q) :  464  -  464
		In Wei Year :  46  Block Range :  236797204  -  242053203  Block reward range (wei) :  464498906012176576041  -  464498906012176576041  Block reward range (q) :  464  -  464
		In Wei Year :  47  Block Range :  242053204  -  247309203  Block reward range (wei) :  464498906012176576041  -  464498906012176576041  Block reward range (q) :  464  -  464
		In Wei Year :  48  Block Range :  247309204  -  252565203  Block reward range (wei) :  464498906012176576041  -  464498906012176576041  Block reward range (q) :  464  -  464
		In Wei Year :  49  Block Range :  252565204  -  257821203  Block reward range (wei) :  232249453006088288021  -  232249453006088288021  Block reward range (q) :  232  -  232
		In Wei Year :  50  Block Range :  257821204  -  263077203  Block reward range (wei) :  232249453006088288021  -  232249453006088288021  Block reward range (q) :  232  -  232
		In Wei Year :  51  Block Range :  263077204  -  268333203  Block reward range (wei) :  232249453006088288021  -  232249453006088288021  Block reward range (q) :  232  -  232
		In Wei Year :  52  Block Range :  268333204  -  273589203  Block reward range (wei) :  232249453006088288021  -  232249453006088288021  Block reward range (q) :  232  -  232
		In Wei Year :  53  Block Range :  273589204  -  278845203  Block reward range (wei) :  116124726503044144010  -  116124726503044144010  Block reward range (q) :  116  -  116
		In Wei Year :  54  Block Range :  278845204  -  284101203  Block reward range (wei) :  116124726503044144010  -  116124726503044144010  Block reward range (q) :  116  -  116
		In Wei Year :  55  Block Range :  284101204  -  289357203  Block reward range (wei) :  116124726503044144010  -  116124726503044144010  Block reward range (q) :  116  -  116
		In Wei Year :  56  Block Range :  289357204  -  294613203  Block reward range (wei) :  116124726503044144010  -  116124726503044144010  Block reward range (q) :  116  -  116
		In Wei Year :  57  Block Range :  294613204  -  299869203  Block reward range (wei) :  58062363251522072005  -  58062363251522072005  Block reward range (q) :  58  -  58
		In Wei Year :  58  Block Range :  299869204  -  305125203  Block reward range (wei) :  58062363251522072005  -  58062363251522072005  Block reward range (q) :  58  -  58
		In Wei Year :  59  Block Range :  305125204  -  310381203  Block reward range (wei) :  58062363251522072005  -  58062363251522072005  Block reward range (q) :  58  -  58
		In Wei Year :  60  Block Range :  310381204  -  315637203  Block reward range (wei) :  58062363251522072005  -  58062363251522072005  Block reward range (q) :  58  -  58
		In Wei Year :  61  Block Range :  315637204  -  320893203  Block reward range (wei) :  29031181625761036003  -  29031181625761036003  Block reward range (q) :  29  -  29
		In Wei Year :  62  Block Range :  320893204  -  326149203  Block reward range (wei) :  29031181625761036003  -  29031181625761036003  Block reward range (q) :  29  -  29
		In Wei Year :  63  Block Range :  326149204  -  331405203  Block reward range (wei) :  29031181625761036003  -  29031181625761036003  Block reward range (q) :  29  -  29
		In Wei Year :  64  Block Range :  331405204  -  336661203  Block reward range (wei) :  29031181625761036003  -  29031181625761036003  Block reward range (q) :  29  -  29
		In Wei Year :  65  Block Range :  336661204  -  341917203  Block reward range (wei) :  14515590812880518001  -  14515590812880518001  Block reward range (q) :  14  -  14
		In Wei Year :  66  Block Range :  341917204  -  347173203  Block reward range (wei) :  14515590812880518001  -  14515590812880518001  Block reward range (q) :  14  -  14
		In Wei Year :  67  Block Range :  347173204  -  352429203  Block reward range (wei) :  14515590812880518001  -  14515590812880518001  Block reward range (q) :  14  -  14
		In Wei Year :  68  Block Range :  352429204  -  357685203  Block reward range (wei) :  14515590812880518001  -  14515590812880518001  Block reward range (q) :  14  -  14
		In Wei Year :  69  Block Range :  357685204  -  362941203  Block reward range (wei) :  7257795406440259001  -  7257795406440259001  Block reward range (q) :  7  -  7
		In Wei Year :  70  Block Range :  362941204  -  368197203  Block reward range (wei) :  7257795406440259001  -  7257795406440259001  Block reward range (q) :  7  -  7
		In Wei Year :  71  Block Range :  368197204  -  373453203  Block reward range (wei) :  7257795406440259001  -  7257795406440259001  Block reward range (q) :  7  -  7
		In Wei Year :  72  Block Range :  373453204  -  378709203  Block reward range (wei) :  7257795406440259001  -  7257795406440259001  Block reward range (q) :  7  -  7
		In Wei Year :  73  Block Range :  378709204  -  383965203  Block reward range (wei) :  3628897703220129500  -  3628897703220129500  Block reward range (q) :  3  -  3
		In Wei Year :  74  Block Range :  383965204  -  389221203  Block reward range (wei) :  3628897703220129500  -  3628897703220129500  Block reward range (q) :  3  -  3
		In Wei Year :  75  Block Range :  389221204  -  394477203  Block reward range (wei) :  3628897703220129500  -  3628897703220129500  Block reward range (q) :  3  -  3
		In Wei Year :  76  Block Range :  394477204  -  399733203  Block reward range (wei) :  3628897703220129500  -  3628897703220129500  Block reward range (q) :  3  -  3
		In Wei Year :  77  Block Range :  399733204  -  404989203  Block reward range (wei) :  1814448851610064750  -  1814448851610064750  Block reward range (q) :  1  -  1
		In Wei Year :  78  Block Range :  404989204  -  410245203  Block reward range (wei) :  1814448851610064750  -  1814448851610064750  Block reward range (q) :  1  -  1
		In Wei Year :  79  Block Range :  410245204  -  415501203  Block reward range (wei) :  1814448851610064750  -  1814448851610064750  Block reward range (q) :  1  -  1
		In Wei Year :  80  Block Range :  415501204  -  420757203  Block reward range (wei) :  1814448851610064750  -  1814448851610064750  Block reward range (q) :  1  -  1
	*/
}

func TestRewardGenerateBlocks1(t *testing.T) {
	startBlockNumber := big.NewInt(500000)
	endBlockNumber := big.NewInt(500005)
	incrementBlock := big.NewInt(1)

	for startBlockNumber.Int64() <= endBlockNumber.Int64() {
		reward := new(big.Int).Set(GetReward(startBlockNumber))
		fmt.Println("Block Number : ", startBlockNumber, " reward : ", reward)
		startBlockNumber = common.SafeAddBigInt(startBlockNumber, incrementBlock)
	}
}

func TestRewardGenerateBlocks(t *testing.T) {
	startBlockNumber := big.NewInt(22338000 - 1000)
	endBlockNumber := big.NewInt(22338000)
	incrementBlock := big.NewInt(1)

	for startBlockNumber.Int64() <= endBlockNumber.Int64() {
		reward := new(big.Int).Set(GetReward(startBlockNumber))
		fmt.Println("Block Number : ", startBlockNumber, " reward : ", reward)
		startBlockNumber = common.SafeAddBigInt(startBlockNumber, incrementBlock)
	}
}

func TestRewardVerifyYearly(t *testing.T) {
	for i := 1; i <= 12; i++ {
		blockNumber := rewardStartBlock.Int64() - 1 + (blockYearly.Int64() * int64(i))
		startBlockNumber := big.NewInt(blockNumber - blockYearly.Int64())
		startReward := new(big.Int).Set(GetReward(startBlockNumber))

		r1 := params.WeiToEther(getTestReward(startBlockNumber))
		r2 := params.WeiToEther(startReward)
		assert.Equal(t, r1, r2)

		endBlockNumber := big.NewInt(blockNumber - 1)
		endReward := new(big.Int).Set(GetReward(endBlockNumber))

		r1 = params.WeiToEther(getTestReward(endBlockNumber))
		r2 = params.WeiToEther(endReward)
		assert.Equal(t, r1, r2)
	}
}

func TestRewardVerifyBlocks(t *testing.T) {
	startBlockNumber := big.NewInt(int64(DefaultConfig.RewardStartBlockNumber) - 1000)
	endBlockNumber := big.NewInt(int64(DefaultConfig.RewardStartBlockNumber - 500))
	incrementBlock := big.NewInt(1)

	for startBlockNumber.Int64() <= endBlockNumber.Int64() {
		reward := new(big.Int).Set(GetReward(startBlockNumber))
		r1 := params.WeiToEther(getTestReward(startBlockNumber))
		r2 := params.WeiToEther(reward)
		assert.Equal(t, r1, r2)
		startBlockNumber = common.SafeAddBigInt(startBlockNumber, incrementBlock)
	}
}

func getTestReward(blockNumber *big.Int) *big.Int {
	var reward = big.NewInt(0)
	if blockStartRang <= blockNumber.Int64() {
		var i = 0
		for i < len(blockEndRange) {
			b := blockEndRange[i] - 1
			if blockNumber.Int64() <= b {
				reward = etherToWeiFloat(big.NewFloat(blockRewardTotal[i]))
				break
			}
			i = i + 1
		}
		return reward
	}
	return reward
}
