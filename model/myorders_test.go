package model

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
	exchange "github.com/preichenberger/go-coinbase-exchange"
)

type MyOrdersTestSuite struct {
	suite.Suite
	mo *MyOrders
}

func (s *MyOrdersTestSuite) SetupTest() {
	s.mo = NewMyOrders(nil, nil)
}

func (s *MyOrdersTestSuite) TestHasBuy() {
	s.mo.myBuys["1"] = exchange.Order{Price: 10.1}
	s.mo.myBuys["2"] = exchange.Order{Price: 10.11}
	assert.Equal(s.T(), s.mo.HasBuyAtPrice(10.1), true)
	assert.Equal(s.T(), s.mo.HasBuyAtPrice(10.2), false)
	assert.Equal(s.T(), s.mo.HasBuyAtPrice(10.111), true)
}

func (s *MyOrdersTestSuite) TestHasSell() {
	s.mo.mySells["1"] = exchange.Order{Price: 10.1}
	s.mo.mySells["2"] = exchange.Order{Price: 10.11}
	assert.Equal(s.T(), s.mo.HasSellAtPrice(10.1), true)
	assert.Equal(s.T(), s.mo.HasSellAtPrice(10.2), false)
	assert.Equal(s.T(), s.mo.HasSellAtPrice(10.111), true)
}

func TestMyOrdersSuite(t *testing.T) {
	suite.Run(t, new(MyOrdersTestSuite))
}
