// Package cart github.com/0magnet/wasm-stuff/pkg/cart/cart.go
package cart

import (
	"syscall/js"
	"strconv"
)

// CartItem represents an item in the cart
type CartItem struct {
	ID     string
	Amount int
}

// saveCart saves the cart to localStorage using JSON.stringify
func saveCart(cart []CartItem) {
	// Convert Go slice to a JavaScript array
	array := js.Global().Call("Array")
	for _, item := range cart {
		obj := js.Global().Call("Object")
		obj.Set("id", item.ID)
		obj.Set("amount", item.Amount)
		array.Call("push", obj)
	}
	// Serialize and save to localStorage
	js.Global().Get("localStorage").Call("setItem", "cartItems", js.Global().Get("JSON").Call("stringify", array))
}

// loadCart loads the cart from localStorage using JSON.parse
func loadCart() []CartItem {
	data := js.Global().Get("localStorage").Call("getItem", "cartItems")
	if data.IsNull() {
		return []CartItem{}
	}

	// Parse JSON string to JavaScript array
	jsArray := js.Global().Get("JSON").Call("parse", data.String())
	cart := []CartItem{}

	// Convert JavaScript array to Go slice
	length := jsArray.Length()
	for i := 0; i < length; i++ {
		jsObj := jsArray.Index(i)
		item := CartItem{
			ID:     jsObj.Get("id").String(),
			Amount: jsObj.Get("amount").Int(),
		}
		cart = append(cart, item)
	}

	return cart
}

// UpdateCartDisplay updates the cart UI
func UpdateCartDisplay() {
	doc := js.Global().Get("document")
	cart := loadCart()
	cartItemsContainer := doc.Call("getElementById", "cart-items")
	totalPriceElement := doc.Call("getElementById", "total-price")
	checkoutButton := doc.Call("getElementById", "checkout-button")

	// Clear cart items
	cartItemsContainer.Set("innerHTML", "")

	// Create table structure
	table := doc.Call("createElement", "table")
	thead := doc.Call("createElement", "thead")
	tbody := doc.Call("createElement", "tbody")

	thead.Set("innerHTML", `
		<tr>
			<th style="text-align: left;">Item</th>
			<th style="text-align: left;">Price</th>
			<th style="text-align: left;">Quantity</th>
			<th style="text-align: left;">Actions</th>
		</tr>
	`)
	table.Call("appendChild", thead)

	// Summarize cart items
	cartSummary := map[string]int{}
	total := 0
	for _, item := range cart {
		cartSummary[item.ID] += item.Amount
		total += item.Amount
	}

	// Render cart items
	for id, amount := range cartSummary {
		row := doc.Call("createElement", "tr")

		productName := "<a href='/p/" + id + "' target='_blank'>" + id + "</a>"

		row.Set("innerHTML", `
			<td style="text-align: left;">`+productName+`</td>
			<td style="text-align: left;">$`+strconv.FormatFloat(float64(amount)/100, 'f', 2, 64)+`</td>
			<td style="text-align: left;">1</td>
			<td style="text-align: left;">
				<button onclick="removeFromCart('`+id+`')">Remove</button>
			</td>
		`)
		tbody.Call("appendChild", row)
	}

	table.Call("appendChild", tbody)
	cartItemsContainer.Call("appendChild", table)

	// Update total price and checkout button
	totalPriceElement.Set("textContent", "Total: $"+strconv.FormatFloat(float64(total)/100, 'f', 2, 64))
	checkoutButton.Set("disabled", total == 0)
}

// AddToCart adds an item to the cart
func AddToCart(this js.Value, args []js.Value) interface{} {
	id := args[0].String()
	amount := args[1].Int()

	cart := loadCart()
	cart = append(cart, CartItem{ID: id, Amount: amount})
	saveCart(cart)
	UpdateCartDisplay()
	return nil
}

// RemoveFromCart removes an item from the cart
func RemoveFromCart(this js.Value, args []js.Value) interface{} {
	id := args[0].String()

	cart := loadCart()
	newCart := []CartItem{}
	for _, item := range cart {
		if item.ID != id {
			newCart = append(newCart, item)
		}
	}
	saveCart(newCart)
	UpdateCartDisplay()
	return nil
}

// EmptyCart clears the cart
func EmptyCart(this js.Value, args []js.Value) interface{} {
	js.Global().Get("localStorage").Call("removeItem", "cartItems")
	UpdateCartDisplay()
	return nil
}

// Initialize initializes the cart system
func Initialize() {
	UpdateCartDisplay()
}

/* //USAGE:
package main

import (
	"syscall/js"
	"github.com/0magnet/wasm-stuff/pkg/cart"
)

func main() {
	// Bind functions to JavaScript
	js.Global().Set("addToCart", js.FuncOf(cart.AddToCart))
	js.Global().Set("removeFromCart", js.FuncOf(cart.RemoveFromCart))
	js.Global().Set("emptyCart", js.FuncOf(cart.EmptyCart))

	// Initialize shopping cart
	js.Global().Get("document").Call("addEventListener", "DOMContentLoaded", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		cart.Initialize()
		return nil
	}))

	// Keep the Go program running
	select {}
}

*/
