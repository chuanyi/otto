package otto

func (self *_runtime) evaluateTryCatch(node *_tryCatchNode) Value {
	tryValue, throw, throwValue := self.tryEvaluate(func() Value { return self.evaluate(node.Try) })

	if throw != false && node.Catch != nil {
		lexicalEnvironment := self._executionContext(0).newDeclarativeEnvironment()
		defer func(){
			self._executionContext(0).LexicalEnvironment = lexicalEnvironment
		}()
		// TODO If necessary, convert TypeError<runtime> => TypeError
		// That, is, such errors can be thrown despite not being JavaScript "native"
		self.localSet(node.Catch.Identifier, throwValue)
		self.evaluate(node.Catch.Body)
	}

	if node.Finally != nil {
		self.evaluate(node.Finally)
		if throw {
			self.Throw(throwValue)
		}
	}

	return tryValue
}

func (self *_runtime) evaluateVariableDeclarationList(node *_variableDeclarationListNode) Value {
	for _, node := range node.VariableList {
		self.evaluateVariableDeclaration(node)
	}
	return emptyValue()
}

func (self *_runtime) evaluateVariableDeclaration(node *_variableDeclarationNode) Value {
	if node.Operator != "" {
		// FIXME If reference is nil
		left := getIdentifierReference(self.LexicalEnvironment(), node.Identifier, false)
		right := self.evaluate(node.Initializer)
		rightValue := self.GetValue(right)

		self.PutValue(left, rightValue)
	}
	return toValue(node.Identifier)
}

func (self *_runtime) evaluateThrow(node *_throwNode) Value {

	self.Throw(self.GetValue(self.evaluate(node.Argument)))

	return UndefinedValue()
}

func (self *_runtime) evaluateReturn(node *_returnNode) Value {
	value := UndefinedValue()
	if node.Argument != nil {
		value = self.GetValue(self.evaluate(node.Argument))
	}

	self.Return(value) // This will panic a resultReturn

	return UndefinedValue()
}

func (self *_runtime) evaluateIf(node *_ifNode) Value {
	test := self.evaluate(node.Test)
	testValue := self.GetValue(test)
	if toBoolean(testValue) {
		return self.evaluate(node.Consequent)
	} else if node.Alternate != nil {
		return self.evaluate(node.Alternate)
	}

	return emptyValue()
}

func (self *_runtime) evaluateWith(node *_withNode) Value {
	object := self.evaluate(node.Object)
	objectValue := self.GetValue(object)
	previousLexicalEnvironment, lexicalEnvironment := self._executionContext(0).newLexicalEnvironment(self.toObject(objectValue))
	lexicalEnvironment.ProvideThis = true
	defer func(){
		self._executionContext(0).LexicalEnvironment = previousLexicalEnvironment
	}()

	return self.evaluate(node.Body)
}

func (self *_runtime) evaluateDoWhile(node *_doWhileNode) Value {

	test := node.Test
	body := node.Body
	_labelSet := node._labelSet

	return self.breakEvaluate(_labelSet, func() Value {
		result := emptyValue()
		for {
			value, skip := self.continueEvaluate(body, _labelSet)
			if skip {
				continue
			}
			if !value.isEmpty() {
				result = value
			}
			testResult := self.evaluate(test)
			testResultValue := self.GetValue(testResult)
			if (toBoolean(testResultValue) == false) {
				break
			}
		}
		return result
	})
}

func (self *_runtime) evaluateWhile(node *_whileNode) Value {

	test := node.Test
	body := node.Body
	_labelSet := node._labelSet

	return self.breakEvaluate(_labelSet, func() Value {
		result := emptyValue()
		for {
			testResult := self.evaluate(test)
			testResultValue := self.GetValue(testResult)
			if (toBoolean(testResultValue) == false) {
				break
			}
			value, _ := self.continueEvaluate(body, _labelSet)
			if !value.isEmpty() {
				result = value
			}
		}
		return result
	})
}

func (self *_runtime) evaluateFor(node *_forNode) Value {

	initial := node.Initial
	test := node.Test
	update := node.Update
	body := node.Body
	_labelSet := node._labelSet

	if initial != nil {
		initialResult := self.evaluate(initial)
		self.GetValue(initialResult) // Side-effect trigger
	}

	return self.breakEvaluate(_labelSet, func() Value {
		result := emptyValue()
		for {
			testResult := self.evaluate(test)
			testResultValue := self.GetValue(testResult)
			if (toBoolean(testResultValue) == false) {
				break
			}
			value, _ := self.continueEvaluate(body, _labelSet)
			if !value.isEmpty() {
				result = value
			}
			updateResult := self.evaluate(update)
			self.GetValue(updateResult) // Side-effect trigger
		}
		return result
	})
}

func (self *_runtime) evaluateForIn(node *_forInNode) Value {

	source := self.evaluate(node.Source)
	sourceValue := self.GetValue(source)

	switch sourceValue._valueType {
	case valueUndefined, valueNull:
		return emptyValue()
	}

	sourceObject := self.toObject(sourceValue)

	into := node.Into
	body := node.Body
	_labelSet := node._labelSet

	return self.breakEvaluate(_labelSet, func() Value {
		result := emptyValue()
		object := sourceObject
		for object != nil {
			object.Enumerate(func(name string){
				into := self.evaluate(into)
				// In the case of: for (var abc in def) ...
				if into.reference() == nil {
					identifier := toString(into)
					// TODO Should be true or false (strictness) depending on context
					into = toValue(getIdentifierReference(self.LexicalEnvironment(), identifier, false))
				}
				self.PutValue(into.reference(), toValue(name))
				value, _ := self.continueEvaluate(body, _labelSet)
				if !value.isEmpty() {
					result = value
				}
			})
			object = object.Prototype
		}
		return result
	})
}

func (self *_runtime) evaluateSwitch(node *_switchNode) Value {

	discriminantResult := self.evaluate(node.Discriminant)

	target := node.Default
	for index, clause := range node.CaseList {
		test := clause.Test
		if test != nil {
			testResult := self.evaluate(test)
			if self.calculateComparison("===", discriminantResult, testResult) {
				target = index
				break
			}
		}
	}

	if target != -1 {
		for _, clause := range node.CaseList[target:] {
			self.evaluateBody(clause.Body)
		}
	}

	return emptyValue()
}