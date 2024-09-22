package net

import (
	"fmt"
	"net/http"

	"firebase.google.com/go/auth"
)

func handleSomething() http.HandlerFunc {
	type request struct {
		Name string
	}
	type response struct {
		Greeting string `json:"greeting"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}
}

// we don't need login becase we are using firebase auth
//
//	{
//		uid: '123456789',
//		email: 'user@example.com',
//		emailVerified: true,
//		password: 'password',
//		multiFactor: {
//		  enrolledFactors: [
//			// When creating users with phone second factors, the uid and
//			// enrollmentTime should not be specified. These will be provisioned by
//			// the Auth server.
//			// Primary second factor.
//			{
//			  phoneNumber: '+16505550001',
//			  displayName: 'Corp phone',
//			  factorId: 'phone',
//			},
//			// Backup second factor.
//			{
//			  phoneNumber: '+16505550002',
//			  displayName: 'Personal phone',
//			  factorId: 'phone'
//			},
//		  ],
//		},
//	  }
func handleRegister(c *auth.Client) http.HandlerFunc {

	// TODO: mfa
	// type MultiFactor struct {
	// 	EnrolledFactors []struct {
	// 		PhoneNumber string `json:"phoneNumber"`
	// 		DisplayName string `json:"displayName"`
	// 		FactorId    string `json:"factorId"`
	// 	} `json:"enrolledFactors"`
	// }

	// UID should not be specified if user is created with MFA
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		// MultiFactor   MultiFactor `json:"multiFactor"`
	}

	// using response type from auth.UserRecord

	return func(w http.ResponseWriter, r *http.Request) {
		// remove client from context
		// TODO, you should probably change how you pass in the client to the handler
		params, err := decode[request](r)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		record, err := c.CreateUser(r.Context(), (&auth.UserToCreate{}).
			Email(params.Email).
			Password(params.Password),
		)
		if err != nil {
			// could not create user error
			fmt.Println(err)
			http.Error(w, "Could not create user", http.StatusInternalServerError)
			return
		}
		encode[auth.UserRecord](w, r, http.StatusOK, *record)
		return
	}
}
